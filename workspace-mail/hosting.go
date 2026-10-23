// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fahimc/stalwart/workspace-mail/internal/platform"
)

func (a *App) hostingProgress(phase string) { a.mu.Lock(); a.hostingPhase = phase; a.mu.Unlock() }

type DNSConflict struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Existing string `json:"existing"`
	Proposed string `json:"proposed"`
}
type HostingIssue struct {
	Code       string        `json:"code"`
	Title      string        `json:"title"`
	Domain     string        `json:"domain"`
	Conflicts  []DNSConflict `json:"conflicts"`
	Resolution string        `json:"resolution,omitempty"`
}

func (i *HostingIssue) Error() string {
	return i.Title + " for " + i.Domain + ". Existing DNS is preserved. Open Mail settings for the conflicting records and recovery options."
}

func (a *App) beginHosting(parent context.Context) {
	a.mu.Lock()
	if a.hostingBusy || a.cfg.MainMailURL != "" {
		a.mu.Unlock()
		return
	}
	a.hostingBusy = true
	a.hostingError = ""
	a.hostingIssue = nil
	c := a.cfg
	a.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(parent, 4*time.Minute)
		defer cancel()
		a.hostingProgress("Creating Mail and connecting your Workspace account…")
		var e error
		if !c.Configured && !c.IndependentMail {
			if main := a.suggestSharedMail(ctx, c, ""); main.Ready {
				e = &HostingIssue{Code: "main_mail_available", Title: "An existing Mail service is available", Domain: main.Domain, Resolution: main.URL}
			}
		}
		if e == nil && !c.Configured {
			e = a.configureMail(ctx, c)
			if e != nil {
				a.stopEngine()
			}
		}
		if e == nil {
			e = a.configureHosting(ctx)
			var conflict *HostingIssue
			if errors.As(e, &conflict) {
				a.hostingProgress("Keeping existing email safe and finding an available mail address…")
				a.mu.RLock()
				current := a.cfg
				a.mu.RUnlock()
				plan, planErr := a.chooseMailDomain(ctx, current)
				if planErr == nil {
					a.hostingProgress("Preparing @" + plan.Domain + " without changing existing email…")
					planErr = a.applyMailDomain(ctx, plan.Domain, current.Domain, true)
					if planErr == nil {
						e = a.configureHosting(ctx)
					}
				}
				if planErr != nil {
					conflict.Resolution = planErr.Error()
					e = conflict
				}
			}
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		a.hostingBusy = false
		if e != nil {
			a.hostingError = e.Error()
			var issue *HostingIssue
			if errors.As(e, &issue) {
				a.hostingIssue = issue
			}
			a.hostingPhase = "Mail hosting needs attention"
		} else {
			// DNS and local configuration cannot establish reachability through NAT,
			// an ISP block, or a provider's reverse-DNS controls.
			a.hostingPhase = "Mail DNS is published. Automatic signing keys and TLS renewal are enabled; certificate issuance may still be in progress. Internet delivery needs an externally reachable TCP port 25 and matching reverse DNS from your server provider."
		}
	}()
}

func (a *App) retryHosting(w http.ResponseWriter, r *http.Request) {
	if !a.setupAllowed(w, r) {
		return
	}
	a.mu.RLock()
	managed := a.cfg.AutomaticHosting && a.cfg.MainMailURL == ""
	a.mu.RUnlock()
	if !managed {
		fail(w, errors.New("automatic hosting is managed by the Workspace installer"))
		return
	}
	a.inboundProgress("checking", "Rechecking the incoming SMTP network route…")
	select {
	case a.inboundRetry <- struct{}{}:
	default:
	}
	a.beginHosting(context.Background())
	w.WriteHeader(http.StatusAccepted)
}

type hostedRecord struct {
	ID     string    `json:"id"`
	Record DNSRecord `json:"record"`
	Reused bool      `json:"reused,omitempty"`
}
type hostingState struct {
	Domain  string         `json:"domain"`
	Zone    string         `json:"zone"`
	Records []hostedRecord `json:"records"`
}

func (a *App) configureHosting(ctx context.Context) error {
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	a.mu.RLock()
	c := a.cfg
	a.mu.RUnlock()
	if c.MainMailURL != "" {
		return errors.New("This PC uses the main Mail service")
	}
	if !c.Configured || !zonePattern.MatchString(c.DNSZone) || c.DNSToken == "" {
		return errors.New("Workspace could not pass the domain connection to Mail; reopen the installer connection step")
	}
	a.hostingProgress("Discovering mail DNS and this computer’s public address…")
	var zone struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if e := cloudflareCall(ctx, c.DNSToken, "GET", "/zones/"+c.DNSZone, nil, &zone); e != nil {
		return e
	}
	if zone.Status != "active" || (c.Domain != zone.Name && !strings.HasSuffix(c.Domain, "."+zone.Name)) {
		return errors.New("the connected Cloudflare domain is not active or does not match Mail")
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api4.ipify.org", nil)
	resp, e := (&http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
	if e != nil {
		return errors.New("could not discover this server’s public IPv4 address; Mail is available but Internet hosting is paused")
	}
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 128))
	resp.Body.Close()
	ip := net.ParseIP(strings.TrimSpace(string(raw)))
	if readErr != nil || resp.StatusCode != 200 || ip == nil || ip.To4() == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return errors.New("this network did not provide a usable public IPv4 address; Mail hosting needs a reachable server or gateway")
	}
	records := []DNSRecord{
		{Type: "A", Name: c.EngineHostname, Content: ip.String(), TTL: 300},
		{Type: "MX", Name: c.Domain, Content: c.EngineHostname, Priority: 10, TTL: 300},
		{Type: "TXT", Name: c.Domain, Content: "v=spf1 mx -all", TTL: 300},
		{Type: "TXT", Name: "_dmarc." + c.Domain, Content: "v=DMARC1; p=none", TTL: 300},
	}
	state := hostingState{Domain: c.Domain, Zone: c.DNSZone}
	statePath := filepath.Join(a.data, "hosting-dns.json")
	if e = platform.LoadJSON(statePath, &state); e != nil && !os.IsNotExist(e) {
		return e
	}
	if state.Domain != c.Domain || state.Zone != c.DNSZone {
		return errors.New("saved hosting DNS belongs to a different domain; automatic migration is disabled")
	}
	// Preflight every RRSet before publishing anything. Only records created by
	// this installation can be reused; existing mail services require migration.
	pending := []DNSRecord{}
	issue := &HostingIssue{Code: "existing_mail_dns", Title: "Finish connecting Internet mail", Domain: c.Domain}
	for _, record := range records {
		type dnsItem struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Name     string `json:"name"`
			Content  string `json:"content"`
			Proxied  bool   `json:"proxied"`
			Priority int    `json:"priority"`
		}
		var existing []dnsItem
		for page := 1; page <= 100; page++ {
			var batch []dnsItem
			if e = cloudflareCall(ctx, c.DNSToken, "GET", "/zones/"+c.DNSZone+"/dns_records?name="+url.QueryEscape(record.Name)+fmt.Sprintf("&per_page=100&page=%d", page), nil, &batch); e != nil {
				return e
			}
			existing = append(existing, batch...)
			if len(batch) < 100 {
				break
			}
			if page == 100 {
				return errors.New("too many existing DNS records to safely configure this domain automatically")
			}
		}
		found := false
		for _, item := range existing {
			relevant := record.Name == c.EngineHostname || item.Type == record.Type
			if record.Type == "TXT" && record.Name == c.Domain {
				relevant = item.Type == "TXT" && strings.HasPrefix(strings.ToLower(strings.Trim(item.Content, "\"")), "v=spf1") || item.Type == "CNAME"
			}
			if item.Type == "CNAME" {
				relevant = true
			}
			if !relevant {
				continue
			}
			owned := false
			for _, saved := range state.Records {
				if saved.ID == item.ID && saved.Record.Name == record.Name && saved.Record.Type == record.Type {
					owned = true
					break
				}
			}
			// An exact compatible record is safe to use without editing it. A receipt
			// records reuse explicitly; it is not permission to replace DNS later.
			compatible := item.Type == record.Type && strings.EqualFold(strings.TrimSuffix(strings.Trim(item.Content, "\""), "."), strings.TrimSuffix(record.Content, ".")) && !item.Proxied && item.Priority == record.Priority
			if !owned && compatible {
				found = true
				state.Records = append(state.Records, hostedRecord{ID: item.ID, Record: record, Reused: true})
				continue
			}
			if !owned {
				issue.Conflicts = append(issue.Conflicts, DNSConflict{Type: item.Type, Name: item.Name, Existing: item.Content, Proposed: record.Content})
				continue
			}
			if !compatible {
				return fmt.Errorf("the managed DNS record for %s changed; review it before retrying hosting", record.Name)
			}
			found = true
		}
		if !found {
			pending = append(pending, record)
		}
	}
	if len(issue.Conflicts) > 0 {
		return issue
	}
	// Persist reuse even when every required record already exists. This does not
	// grant permission to alter those records on a later setup attempt.
	if e = platform.SaveJSON(statePath, state); e != nil {
		return e
	}
	a.hostingProgress("Publishing mail DNS without replacing existing records…")
	for _, record := range pending {
		var created struct {
			ID string `json:"id"`
		}
		if e = cloudflareCall(ctx, c.DNSToken, "POST", "/zones/"+c.DNSZone+"/dns_records", record, &created); e != nil {
			return e
		}
		state.Records = append(state.Records, hostedRecord{ID: created.ID, Record: record})
		if e = platform.SaveJSON(statePath, state); e != nil {
			return fmt.Errorf("DNS was created but its ownership receipt could not be saved; inspect Cloudflare before retrying: %w", e)
		}
	}
	a.hostingProgress("Generating signing keys and enabling automatic certificates…")
	var recovery map[string]string
	if e = platform.LoadSecret(filepath.Join(a.data, "engine.dpapi"), &recovery); e != nil {
		return e
	}
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:"+recovery["password"]))
	// Wait for the normal engine after the bootstrap restart.
	for n := 0; n < 60; n++ {
		_, e = a.jmap(ctx, auth, "x:Domain/get", map[string]any{})
		if e == nil {
			break
		}
		select {
		case <-ctx.Done():
			return errors.New("Mail engine startup timed out")
		case <-time.After(500 * time.Millisecond):
		}
	}
	if e != nil {
		return e
	}
	dnsID, e := a.hostingObject(ctx, auth, "DnsServer", "description", "Workspace automatic hosting", map[string]any{"@type": "Cloudflare", "description": "Workspace automatic hosting", "secret": map[string]any{"@type": "EnvironmentVariable", "variableName": "WORKSPACE_MAIL_DNS_TOKEN"}})
	if e != nil {
		return e
	}
	acmeID, e := a.hostingObject(ctx, auth, "AcmeProvider", "directory", "https://acme-v02.api.letsencrypt.org/directory", map[string]any{"challengeType": "Dns01", "contact": map[string]bool{"postmaster@" + c.Domain: true}, "directory": "https://acme-v02.api.letsencrypt.org/directory"})
	if e != nil {
		return e
	}
	domains, e := a.jmap(ctx, auth, "x:Domain/get", map[string]any{})
	if e != nil {
		return e
	}
	list, _ := domains["list"].([]any)
	for _, item := range list {
		domain, ok := item.(map[string]any)
		if !ok || domain["name"] != c.Domain {
			continue
		}
		id, _ := domain["id"].(string)
		sum := sha256.Sum256([]byte(a.data + "|" + c.Domain))
		selector := "workspace-" + hex.EncodeToString(sum[:6]) + "-{algorithm}-{date-%Y%m%d}"
		patch := map[string]any{
			"dnsManagement":         map[string]any{"@type": "Automatic", "dnsServerId": dnsID, "origin": zone.Name, "publishRecords": map[string]bool{"dkim": true}},
			"certificateManagement": map[string]any{"@type": "Automatic", "acmeProviderId": acmeID, "subjectAlternativeNames": map[string]bool{c.EngineHostname: true}},
			"dkimManagement":        map[string]any{"@type": "Automatic", "selectorTemplate": selector},
		}
		_, e = a.jmap(ctx, auth, "x:Domain/set", map[string]any{"update": map[string]any{id: patch}})
		return e
	}
	return errors.New("Mail’s managed domain was not found in Stalwart")
}

func (a *App) hostingObject(ctx context.Context, auth, kind, key, value string, create map[string]any) (string, error) {
	objects, e := a.jmap(ctx, auth, "x:"+kind+"/get", map[string]any{})
	if e != nil {
		return "", e
	}
	list, _ := objects["list"].([]any)
	for _, item := range list {
		if obj, ok := item.(map[string]any); ok && obj[key] == value {
			id, _ := obj["id"].(string)
			return id, nil
		}
	}
	result, e := a.jmap(ctx, auth, "x:"+kind+"/set", map[string]any{"create": map[string]any{"workspace": create}})
	if e != nil {
		return "", e
	}
	created, _ := result["created"].(map[string]any)
	obj, _ := created["workspace"].(map[string]any)
	id, _ := obj["id"].(string)
	if id == "" {
		return "", fmt.Errorf("Stalwart did not create the %s hosting settings", kind)
	}
	return id, nil
}
