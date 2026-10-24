// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fahimc/stalwart/workspace-mail/internal/platform"
	"github.com/oneworkspacestudio/calendar/shared"
)

//go:embed assets/*
var bundled embed.FS
var version = "0.1.0-preview.1"

type Config struct {
	MainMailURL      string `json:"mainMailURL,omitempty"`
	IndependentMail  bool   `json:"independentMail,omitempty"`
	Hostname         string `json:"hostname"`
	Domain           string `json:"domain"`
	Origin           string `json:"origin"`
	Issuer           string `json:"issuer"`
	ClientID         string `json:"clientId"`
	ClientSecret     string `json:"clientSecret"`
	OwnerSubject     string `json:"ownerSubject"`
	EnginePort       int    `json:"enginePort"`
	SMTPPort         int    `json:"smtpPort"`
	OIDCClaim        string `json:"oidcClaim"`
	Configured       bool   `json:"configured"`
	RelayHost        string `json:"relayHost"`
	RelayPort        int    `json:"relayPort"`
	RelayUsername    string `json:"relayUsername"`
	RelaySecret      string `json:"relaySecret"`
	EngineSchema     int    `json:"engineSchema"`
	EngineHostname   string `json:"engineHostname,omitempty"`
	AutomaticHosting bool   `json:"automaticHosting,omitempty"`
	DNSZone          string `json:"dnsZone,omitempty"`
	DNSToken         string `json:"dnsToken,omitempty"`
}
type Session struct {
	Auth, Subject, Name, Email, CSRF string
	Expires                          time.Time
	TokenExpiry                      time.Time
	RefreshToken                     string
	mu                               sync.Mutex
}
type App struct {
	sharedRoot                 string
	data, base                 string
	port                       int
	mu                         sync.RWMutex
	cfg                        Config
	sessions                   map[string]*Session
	pending                    map[string]Login
	setupToken                 string
	job                        *platform.Job
	engine                     *exec.Cmd
	held                       *os.File
	client                     *http.Client
	sharedProbeMu              sync.Mutex
	sharedProbe                sharedMailPlan
	sharedProbeAt              time.Time
	networkMu                  sync.Mutex
	networkCancel              context.CancelFunc
	setupMu                    sync.Mutex
	provisionMu                sync.Mutex
	provisionError             string
	inboundState               string
	inboundMessage             string
	inboundHelp                inboundHelp
	smtpObservation            smtpObservation
	probeLoggingReady          bool
	inboundRetry               chan struct{}
	engineDone                 chan struct{}
	restarting                 bool
	hostingBusy                bool
	hostingPhase, hostingError string
	hostingIssue               *HostingIssue
	identityError              string
}

func main() {
	data := flag.String("data", filepath.Join(os.Getenv("LOCALAPPDATA"), "OneWorkspace", "Mail"), "private NTFS data folder")
	port := flag.Int("port", 19496, "loopback web port")
	base := flag.String("base", os.Getenv("WORKSPACE_BASE_PATH"), "external path prefix, e.g. /mail")
	noBrowser := flag.Bool("no-browser", false, "do not open browser")
	origin := flag.String("origin", "", "public HTTPS origin for standalone hosting")
	openSetup := flag.Bool("setup", false, "open the existing protected setup wizard")
	sharedData := flag.String("shared-data", shared.DefaultRoot(), "shared Stalwart installation data")
	legacy := flag.Bool("legacy-engine", privateEngineDefault(), "explicitly run an existing legacy private engine; never auto-migrate")
	flag.Parse()
	sharedExplicit, legacyExplicit := false, false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "shared-data" {
			sharedExplicit = true
		}
		if f.Name == "legacy-engine" {
			legacyExplicit = true
		}
	})
	if sharedExplicit && !legacyExplicit && os.Getenv("WORKSPACE_MAIL_BACKEND") != "private" {
		*legacy = false
	}
	if *port < 1024 || *port > 65532 {
		log.Fatal("Choose a port from 1024 to 65532")
	}
	absolute, e := filepath.Abs(*data)
	if e != nil {
		log.Fatal(e)
	}
	if e = os.MkdirAll(absolute, 0700); e != nil {
		log.Fatal(e)
	}
	if e = platform.ProtectDir(absolute); e != nil {
		log.Fatal(e)
	}
	if *openSetup {
		var saved map[string]string
		if e = platform.LoadSecret(filepath.Join(absolute, "setup.dpapi"), &saved); e != nil {
			log.Fatal("Start Mail once before reopening setup")
		}
		platform.OpenBrowser(saved["url"])
		return
	}
	lock, e := platform.Lock(filepath.Join(absolute, "runtime.lock"))
	if e != nil {
		log.Fatal(e)
	}
	defer lock.Close()
	job, e := platform.NewJob()
	if e != nil {
		log.Fatal(e)
	}
	defer job.Close()
	app := &App{data: absolute, base: strings.TrimRight(*base, "/"), port: *port, sessions: map[string]*Session{}, pending: map[string]Login{}, job: job, inboundRetry: make(chan struct{}, 1), client: &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	app.cfg = Config{Hostname: "mail.example.com", Domain: "example.com", Origin: fmt.Sprintf("http://localhost:%d", *port), EnginePort: *port + 1, SMTPPort: *port + 2, OIDCClaim: "sub"}
	if e = platform.LoadSecret(filepath.Join(absolute, "mail.dpapi"), &app.cfg); e != nil && !os.IsNotExist(e) {
		log.Fatal("Cannot read protected Mail configuration: ", e)
	}
	if *origin != "" {
		if !validURL(*origin) {
			log.Fatal("Invalid public origin")
		}
		app.cfg.Origin = strings.TrimRight(*origin, "/")
	}
	if v := os.Getenv("WORKSPACE_OIDC_ISSUER"); v != "" {
		app.cfg.Issuer = v
		app.cfg.ClientID = os.Getenv("WORKSPACE_OIDC_CLIENT_ID")
		app.cfg.ClientSecret = os.Getenv("WORKSPACE_OIDC_CLIENT_SECRET")
		app.cfg.OwnerSubject = os.Getenv("WORKSPACE_OWNER_SUBJECT")
		if u, e := url.Parse(os.Getenv("WORKSPACE_OIDC_REDIRECT_URI")); e == nil && u.Host != "" {
			app.cfg.Origin = u.Scheme + "://" + u.Host
		}
	}
	if app.cfg.MainMailURL != "" {
		if _, err := mainMailURL(app.cfg.MainMailURL, app.cfg.Origin); err != nil {
			log.Fatal("Invalid saved main Mail address: ", err)
		}
	}
	if !*legacy && app.cfg.MainMailURL == "" {
		app.sharedRoot = *sharedData
		if _, legacyErr := os.Stat(filepath.Join(app.data, "engine.json")); legacyErr == nil {
			log.Fatal("Legacy private Mail storage requires an explicit administrator migration; no shared backend was adopted.")
		}
		if e = app.attachShared(context.Background()); e != nil && !os.IsNotExist(e) {
			log.Fatal(e)
		} else if e != nil && app.cfg.Configured {
			log.Fatal("Existing private Mail data was found. Use an explicit administrator migration or --legacy-engine; shared storage was not adopted.")
		}
	}
	if domain := os.Getenv("WORKSPACE_MAIL_DOMAIN"); domain != "" && !app.cfg.Configured {
		if !domainPattern.MatchString(domain) || len(domain) > 220 || app.cfg.Issuer == "" {
			log.Fatal("Invalid managed Mail domain or missing MyAccounts connection")
		}
		u, _ := url.Parse(app.cfg.Origin)
		app.cfg.Domain = domain
		app.cfg.Hostname = u.Hostname()
		app.cfg.EngineHostname = "smtp." + domain
		app.cfg.AutomaticHosting = true
		app.cfg.DNSZone = os.Getenv("WORKSPACE_MAIL_DNS_ZONE")
		app.cfg.DNSToken = os.Getenv("WORKSPACE_MAIL_DNS_TOKEN")
	}
	app.setupToken = platform.Token()
	if e = platform.SaveSecret(filepath.Join(absolute, "setup.dpapi"), map[string]string{"url": app.cfg.Origin + app.base + "/setup#" + app.setupToken}); e != nil {
		log.Fatal(e)
	}
	if app.cfg.Configured && app.sharedRoot == "" && app.cfg.MainMailURL == "" {
		if e = app.startEngine(); e != nil {
			log.Fatal(e)
		}
		resumeCtx, resumeCancel := context.WithTimeout(context.Background(), 90*time.Second)
		_, resumeErr := app.resumeMailDomainChange(resumeCtx)
		resumeCancel()
		if resumeErr != nil {
			app.identityError = "Mail-domain change needs attention: " + resumeErr.Error()
			log.Print(app.identityError)
		}
		if app.cfg.EngineSchema < 2 && app.cfg.Issuer != "" && app.identityError == "" {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			e = app.configureIdentity(ctx)
			cancel()
			if e != nil {
				// Keep the recovery UI available if a custom policy needs an owner.
				app.identityError = "Mailbox access setup needs attention: " + e.Error()
				log.Print(app.identityError)
			} else {
				app.cfg.EngineSchema = 2
				if e = platform.SaveSecret(filepath.Join(absolute, "mail.dpapi"), app.cfg); e != nil {
					log.Fatal(e)
				}
				app.stopEngine()
				if e = app.startEngine(); e != nil {
					log.Fatal(e)
				}
			}
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		app.mu.RLock()
		ready := app.cfg.Configured || app.cfg.MainMailURL != ""
		app.mu.RUnlock()
		writeJSON(w, map[string]any{"status": "ok", "version": version, "configured": ready})
	})
	mux.HandleFunc("GET /api/status", app.status)
	mux.HandleFunc("GET /api/hosting/shared/status", app.sharedMailAvailability)
	mux.HandleFunc("POST /api/hosting/shared/plan", app.planSharedMail)
	mux.HandleFunc("POST /api/hosting/shared/apply", app.applySharedMail)
	mux.HandleFunc("POST /api/setup", app.setup)
	mux.HandleFunc("POST /api/identity/repair", app.repairIdentity)
	mux.HandleFunc("POST /api/mailboxes/provision", app.provisionMailboxes)
	mux.HandleFunc("POST /api/hosting/subdomain/plan", app.planMailDomain)
	mux.HandleFunc("POST /api/hosting/subdomain/apply", app.useMailDomain)
	mux.HandleFunc("POST /api/dns/plan", app.dnsPlan)
	mux.HandleFunc("POST /api/dns/apply", app.dnsApply)
	mux.HandleFunc("POST /api/delivery", app.delivery)
	mux.HandleFunc("POST /api/hosting/retry", app.retryHosting)
	mux.HandleFunc("POST /api/hosting/network/retry", app.retryIncoming)
	mux.HandleFunc("GET /auth/login", app.login)
	mux.HandleFunc("GET /auth/callback", app.callback)
	mux.HandleFunc("POST /auth/password", app.passwordLogin)
	mux.HandleFunc("POST /auth/logout", app.logout)
	mux.HandleFunc("GET /api/session", app.sessionInfo)
	mux.HandleFunc("POST /api/jmap", app.proxyJMAP)
	mux.HandleFunc("POST /api/upload", app.upload)
	mux.HandleFunc("GET /api/download", app.download)
	assets, _ := fs.Sub(bundled, "assets")
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/setup" {
			http.NotFound(w, r)
			return
		}
		b, _ := bundled.ReadFile("assets/index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Workspace strips the prefix; a standalone reverse proxy can preserve it.
		if app.base != "" && (r.URL.Path == app.base || strings.HasPrefix(r.URL.Path, app.base+"/")) {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, app.base)
			if r.URL.Path == "" {
				r.URL.Path = "/"
			}
		}
		app.mu.RLock()
		origin := app.cfg.Origin
		app.mu.RUnlock()
		u, _ := url.Parse(origin)
		localHost := r.Host == fmt.Sprintf("localhost:%d", app.port) || r.Host == fmt.Sprintf("127.0.0.1:%d", app.port)
		if !localHost && r.Host != u.Host {
			http.Error(w, "Unknown host", 403)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-src 'self' blob:; object-src 'none'; base-uri 'none'; frame-ancestors 'self'; form-action 'self'")
		if r.Method != "GET" && r.Method != "HEAD" {
			if r.Header.Get("Origin") != origin && !(localHost && r.Header.Get("Origin") == "http://"+r.Host) {
				http.Error(w, "Origin rejected", 403)
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
	server := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", *port), Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if app.cfg.AutomaticHosting && app.identityError == "" {
		app.beginHosting(ctx)
		go app.maintainInboundMail(ctx)
	}
	go func() { <-ctx.Done(); _ = server.Close() }()
	log.Printf("Mail %s listening on http://localhost:%d%s/; setup link is protected in %s", version, *port, app.base, filepath.Join(absolute, "setup.dpapi"))
	if !*noBrowser {
		if app.cfg.Configured {
			platform.OpenBrowser(app.cfg.Origin + app.base + "/")
		} else {
			platform.OpenBrowser(app.cfg.Origin + app.base + "/setup#" + app.setupToken)
		}
	}
	if e = server.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
		log.Fatal(e)
	}
}
func (a *App) startEngine() error {
	if a.sharedRoot != "" || a.cfg.MainMailURL != "" {
		return nil
	}
	if err := a.startEngineMode(false); err != nil {
		return err
	}
	// Starting a process does not mean its registry and account directory are
	// ready. Never finish setup or expose sign-in while that restart is pending.
	auth, err := a.engineAdmin()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	for {
		if _, err = a.jmap(ctx, auth, "x:Authentication/get", map[string]any{"ids": []string{"singleton"}}); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			log.Printf("Mail engine startup diagnostic: %v", err)
			return errors.New("Mail is still starting. Wait a moment, then choose Reconnect Mail.")
		case <-time.After(250 * time.Millisecond):
		}
	}
}
func (a *App) stopEngine() {
	if a.sharedRoot != "" {
		return
	}
	if a.engine == nil {
		return
	}
	a.mu.Lock()
	a.restarting = true
	a.mu.Unlock()
	_ = a.engine.Process.Kill()
	<-a.engineDone
	a.mu.Lock()
	a.restarting = false
	a.mu.Unlock()
	a.engine = nil
}
func (a *App) startEngineMode(recoveryMode bool) error {
	if a.sharedRoot != "" {
		return errors.New("Stalwart is owned by WorkspaceServices.exe")
	}
	exe, e := loadPrivateEngine()
	if e != nil {
		return e
	}
	sum := sha256.Sum256(exe)
	expected := hex.EncodeToString(sum[:])
	path := filepath.Join(a.data, "runtime", "stalwart-0.16.26.exe")
	if b, e := os.ReadFile(path); e != nil || sha256.Sum256(b) != sum {
		if e = platform.AtomicWrite(path, exe); e != nil {
			return e
		}
	}
	a.held, e = platform.LockedBinary(path, expected)
	if e != nil {
		return e
	}
	var recovery map[string]string
	if e = platform.LoadSecret(filepath.Join(a.data, "engine.dpapi"), &recovery); e != nil {
		a.held.Close()
		return e
	}
	c := a.cfg
	cmd := exec.Command(path, "--config", filepath.Join(a.data, "engine.json"))
	cmd.Dir = a.data
	engineHost := c.EngineHostname
	if engineHost == "" {
		engineHost = c.Hostname
	}
	cmd.Env = append(os.Environ(), "STALWART_RECOVERY_ADMIN=admin:"+recovery["password"], "STALWART_RECOVERY_MODE_PORT="+strconv.Itoa(c.EnginePort), "STALWART_HOSTNAME="+engineHost, "STALWART_PUBLIC_URL="+fmt.Sprintf("http://127.0.0.1:%d", c.EnginePort), "WORKSPACE_MAIL_DNS_TOKEN="+c.DNSToken)
	cmd.Env = append(cmd.Env, "STALWART_RECOVERY_MODE="+strconv.FormatBool(recoveryMode))
	logFile, e := os.OpenFile(filepath.Join(a.data, "engine.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if e != nil {
		a.held.Close()
		return e
	}
	a.smtpObservation.restoreReceipts(filepath.Join(a.data, "engine.log"))
	filtered := &engineLog{observe: a.smtpObservation.observe, out: logFile, secrets: []string{recovery["password"], c.ClientSecret, c.RelaySecret, c.DNSToken}}
	cmd.Stdout = filtered
	cmd.Stderr = filtered
	if e = a.job.Start(cmd); e != nil {
		logFile.Close()
		a.held.Close()
		return e
	}
	a.engine = cmd
	held := a.held
	done := make(chan struct{})
	a.engineDone = done
	go func() {
		e := cmd.Wait()
		logFile.Close()
		held.Close()
		a.mu.RLock()
		intentional := a.restarting
		a.mu.RUnlock()
		close(done)
		if intentional {
			return
		}
		log.Printf("Stalwart stopped (%v); restart Mail to recover", e)
		os.Exit(1)
	}()
	return nil
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, e error) {
	w.Header().Set("Content-Type", "application/json")
	var access *MailAccessError
	if errors.As(e, &access) {
		w.WriteHeader(http.StatusBadGateway)
		code := "mail_unavailable"
		if access.Status == 401 {
			code = "mail_identity_rejected"
		} else if access.Status == 403 {
			code = "mail_access_denied"
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": e.Error(), "code": code})
		return
	}
	w.WriteHeader(400)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": e.Error()})
}
func decode(w http.ResponseWriter, r *http.Request, out any) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(out)
}
