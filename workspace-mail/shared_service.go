// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fahimc/stalwart/workspace-mail/internal/platform"
)

type sharedMailPlan struct {
	URL     string `json:"url"`
	Ready   bool   `json:"ready"`
	Domain  string `json:"domain,omitempty"`
	Issuer  string `json:"issuer,omitempty"`
	Message string `json:"message"`
}

// This is a browser destination, never a proxy or a token-exchange endpoint.
// Restrict metadata probes to HTTPS/443 and pin public DNS answers when dialing.
func mainMailURL(raw, ownOrigin string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || u.Host == "" || (u.Port() != "" && u.Port() != "443") || !domainPattern.MatchString(strings.ToLower(u.Hostname())) {
		return "", errors.New("Enter the main Mail HTTPS address, for example https://workspace.example.com/mail/. Do not include sign-in links or passwords")
	}
	own, _ := url.Parse(ownOrigin)
	if own != nil && strings.EqualFold(own.Hostname(), u.Hostname()) {
		return "", errors.New("Choose the main PC's address, not this Workspace")
	}
	if strings.Contains(u.Path, "\\") || strings.Contains(u.Path, "//") || (u.Path != "" && u.Path != "/" && path.Clean(u.Path) != strings.TrimSuffix(u.Path, "/")) {
		return "", errors.New("Enter a clean Mail address without redirects or path shortcuts")
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/") + "/"
	return u.String(), nil
}

func inspectMainMail(ctx context.Context, target string) sharedMailPlan {
	plan := sharedMailPlan{URL: target, Message: "The main Mail service could not be verified right now. Check that the main PC is running and its address is correct."}
	u, _ := url.Parse(target)
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, u.Hostname())
	if err != nil || len(ips) == 0 {
		return plan
	}
	for _, ip := range ips {
		if !ip.IP.IsGlobalUnicast() || ip.IP.IsPrivate() || ip.IP.IsLoopback() || ip.IP.IsLinkLocalUnicast() || ip.Zone != "" {
			return plan
		}
		if v := ip.IP.To4(); v != nil && v[0] == 100 && v[1] >= 64 && v[1] <= 127 {
			return plan
		}
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		var last error
		for _, ip := range ips {
			conn, e := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(ip.IP.String(), "443"))
			if e == nil {
				return conn, nil
			}
			last = e
		}
		return nil, last
	}, TLSHandshakeTimeout: 4 * time.Second, ResponseHeaderTimeout: 4 * time.Second}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 7 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, _ := http.NewRequestWithContext(ctx, "GET", target+"api/status", nil)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return plan
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized && u.Path == "/mail/" {
		var gate struct {
			Error string `json:"error"`
			Login string `json:"login"`
		}
		if json.NewDecoder(io.LimitReader(resp.Body, 16384)).Decode(&gate) == nil && gate.Error == "Sign in to One Workspace" && gate.Login == "/auth/login" {
			// Older managers protect Mail metadata along with every app request.
			// Confirm only public process availability; preserve the login gate.
			healthURL := *u
			healthURL.Path = "/healthz"
			healthReq, _ := http.NewRequestWithContext(ctx, "GET", healthURL.String(), nil)
			healthReq.Header.Set("Accept", "application/json")
			if healthResp, e := client.Do(healthReq); e == nil {
				defer healthResp.Body.Close()
				var health struct {
					Status string            `json:"status"`
					Apps   map[string]string `json:"apps"`
				}
				if healthResp.StatusCode == http.StatusOK && json.NewDecoder(io.LimitReader(healthResp.Body, 64<<10)).Decode(&health) == nil && health.Status == "ok" && health.Apps["mail"] == "running" {
					plan.Ready = true
					plan.Message = "The main Workspace Mail service is running and requires sign-in. Use its MyAccounts account when Mail opens. Mailbox setup and message delivery are not verified by this availability check."
				}
			}
		}
		return plan
	}
	if resp.StatusCode != http.StatusOK {
		return plan
	}
	var meta struct {
		Configured  bool   `json:"configured"`
		Version     string `json:"version"`
		Domain      string `json:"domain"`
		Issuer      string `json:"issuer"`
		MainMailURL string `json:"mainMailURL"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&meta) != nil || meta.Version == "" || !meta.Configured || !domainPattern.MatchString(meta.Domain) {
		return plan
	}
	if meta.MainMailURL != "" {
		plan.Message = "That PC also connects to another Mail server. Enter the original main Mail address to avoid a forwarding loop."
		return plan
	}
	plan.Ready = true
	plan.Domain = meta.Domain
	if validURL(meta.Issuer) {
		plan.Issuer = meta.Issuer
	}
	plan.Message = "Main Mail is available. Everyone uses their account on that installation; this PC will not receive Internet mail independently."
	return plan
}

func (a *App) planSharedMail(w http.ResponseWriter, r *http.Request) {
	if !a.setupAllowed(w, r) {
		return
	}
	var input struct {
		URL string `json:"url"`
	}
	if err := decode(w, r, &input); err != nil {
		fail(w, err)
		return
	}
	a.mu.RLock()
	c := a.cfg
	a.mu.RUnlock()
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	writeJSON(w, a.suggestSharedMail(ctx, c, input.URL))
}

func (a *App) suggestSharedMail(ctx context.Context, c Config, candidate string) sharedMailPlan {
	if candidate == "" {
		candidate = c.MainMailURL
	}
	if candidate == "" && c.DNSZone != "" && c.DNSToken != "" {
		var zone struct {
			Name string `json:"name"`
		}
		if cloudflareCall(ctx, c.DNSToken, "GET", "/zones/"+c.DNSZone, nil, &zone) == nil && domainPattern.MatchString(zone.Name) {
			// A DNS service pointer wins. Otherwise suggest the standard main Workspace
			// hostname; an owner must still approve the destination. Never infer an
			// identity from a router IP or send credentials during discovery.
			records, _ := net.DefaultResolver.LookupTXT(ctx, "_oneworkspace-mail."+zone.Name)
			for _, record := range records {
				if strings.HasPrefix(record, "oneworkspace-mail=") {
					candidate = strings.TrimPrefix(record, "oneworkspace-mail=")
					break
				}
			}
			if candidate == "" {
				candidate = "https://workspace." + zone.Name + "/mail/"
			}
		}
	}
	if candidate == "" {
		return sharedMailPlan{Message: "Enter the main Workspace Mail address. No existing service could be identified safely."}
	}
	target, err := mainMailURL(candidate, c.Origin)
	if err != nil {
		return sharedMailPlan{Message: err.Error()}
	}
	return inspectMainMail(ctx, target)

}

func (a *App) applySharedMail(w http.ResponseWriter, r *http.Request) {
	if !a.setupAllowed(w, r) {
		return
	}
	var input struct {
		URL              string `json:"url"`
		AllowUnavailable bool   `json:"allowUnavailable"`
	}
	if err := decode(w, r, &input); err != nil {
		fail(w, err)
		return
	}
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	a.mu.RLock()
	old := a.cfg
	busy := a.hostingBusy
	a.mu.RUnlock()
	if busy {
		fail(w, errors.New("Mail setup is finishing. Wait for it to finish, then connect the main service"))
		return
	}
	target := ""
	if input.URL != "" {
		var err error
		target, err = mainMailURL(input.URL, old.Origin)
		if err != nil {
			fail(w, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		plan := inspectMainMail(ctx, target)
		cancel()
		if !plan.Ready && !input.AllowUnavailable {
			fail(w, errors.New(plan.Message))
			return
		}
	}
	// Cancel and join any in-flight mapping/probe before changing the mode.
	a.mu.RLock()
	cancelNetwork := a.networkCancel
	a.mu.RUnlock()
	if cancelNetwork != nil {
		cancelNetwork()
	}
	a.networkMu.Lock()
	defer a.networkMu.Unlock()
	next := old
	next.MainMailURL = target
	next.IndependentMail = target == ""
	if err := platform.SaveSecret(filepath.Join(a.data, "mail.dpapi"), next); err != nil {
		fail(w, err)
		return
	}
	a.mu.Lock()
	a.cfg = next
	a.mu.Unlock()
	if target != "" {
		a.stopEngine()
		a.inboundProgress("shared", "Incoming mail is managed by the main Mail service.")
	} else if old.MainMailURL != "" && next.Configured {
		if err := a.startEngine(); err != nil {
			a.stopEngine()
			a.mu.Lock()
			a.cfg = old
			a.mu.Unlock()
			if restoreErr := platform.SaveSecret(filepath.Join(a.data, "mail.dpapi"), old); restoreErr != nil {
				fail(w, errors.New("Local startup and restoring the shared connection failed. Restart Mail and inspect its protected settings"))
				return
			}
			fail(w, errors.New("Local Mail could not start; the shared connection was kept. Check the server before disconnecting"))
			return
		}
	}
	select {
	case a.inboundRetry <- struct{}{}:
	default:
	}
	if target == "" && old.MainMailURL != target && next.AutomaticHosting {
		a.beginHosting(context.Background())
	}
	if target == "" && !next.Configured && next.AutomaticHosting {
		a.beginHosting(context.Background())
	}
	writeJSON(w, map[string]any{"ok": true, "mainMailURL": target})
}

// Public readiness only, for the one owner-selected destination. Coalesce probes
// to avoid allowing unauthenticated visitors to amplify outgoing requests.
func (a *App) sharedMailAvailability(w http.ResponseWriter, r *http.Request) {
	a.sharedProbeMu.Lock()
	defer a.sharedProbeMu.Unlock()
	a.mu.RLock()
	target := a.cfg.MainMailURL
	a.mu.RUnlock()
	if target == "" {
		writeJSON(w, sharedMailPlan{Message: "This PC hosts its own Mail service."})
		return
	}
	if a.sharedProbe.URL != target || time.Since(a.sharedProbeAt) > 10*time.Second {
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		a.sharedProbe = inspectMainMail(ctx, target)
		a.sharedProbeAt = time.Now()
		cancel()
	}
	writeJSON(w, a.sharedProbe)
}
