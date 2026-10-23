// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/fahimc/stalwart/workspace-mail/internal/platform"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var domainPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)

func (a *App) setupAllowed(w http.ResponseWriter, r *http.Request) bool {
	if a.sharedRoot != "" && a.cfg.Configured {
		http.Error(w, "Shared backend settings are managed by Workspace Services", 409)
		return false
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Setup-Token")), []byte(a.setupToken)) == 1 {
		return true
	}
	s, ok := a.user(w, r)
	if !ok {
		return false
	}
	a.mu.RLock()
	owner := a.cfg.OwnerSubject
	a.mu.RUnlock()
	if owner == "" || s.Subject != owner {
		http.Error(w, "Workspace owner required", 403)
		return false
	}
	return true
}
func (a *App) status(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	c := a.cfg
	hostingBusy, hostingPhase, hostingError := a.hostingBusy, a.hostingPhase, a.hostingError
	issue := a.hostingIssue
	identityError := a.identityError
	if identityError != "" {
		identityError = (&mailIdentitySetupError{}).Error()
	}
	provisionError := a.provisionError
	inboundState, inboundMessage := a.inboundState, a.inboundMessage
	help := a.inboundHelp
	a.mu.RUnlock()
	writeJSON(w, map[string]any{"mainMailURL": c.MainMailURL, "configured": c.Configured, "hostname": c.Hostname, "domain": c.Domain, "origin": c.Origin, "issuer": c.Issuer, "clientId": c.ClientID, "oidc": c.Issuer != "", "base": a.base, "version": version, "callback": c.Origin + a.base + "/auth/callback", "smtpPort": c.SMTPPort, "enginePort": c.EnginePort, "managedHosting": c.AutomaticHosting, "hostingBusy": hostingBusy, "hostingPhase": hostingPhase, "hostingError": hostingError, "hostingIssue": issue, "identityError": identityError, "provisionError": provisionError, "inboundState": inboundState, "inboundHelp": help, "inboundMessage": inboundMessage, "pendingMailDomain": a.pendingMailDomain(), "smtpHostname": c.EngineHostname})
}
func validURL(s string) bool {
	u, e := url.Parse(s)
	return e == nil && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && (u.Scheme == "https" || (u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1")))
}
func (a *App) setup(w http.ResponseWriter, r *http.Request) {
	if !a.setupAllowed(w, r) {
		return
	}
	var c Config
	if e := decode(w, r, &c); e != nil {
		fail(w, e)
		return
	}
	if e := a.configureMail(r.Context(), c); e != nil {
		fail(w, e)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "message": "Mail and MyAccounts are connected.", "callback": c.Origin + a.base + "/auth/callback"})
}
func (a *App) configureMail(parent context.Context, c Config) error {
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	a.mu.RLock()
	old := a.cfg
	a.mu.RUnlock()
	if old.MainMailURL != "" {
		return errors.New("This PC uses the main Mail service. Disconnect it in Mail settings before configuring a local server.")
	}
	if old.Configured {
		return errors.New("This installation is already configured. Use the DNS step to connect its hostname; changing mailbox domains requires an administrator migration.")
	}
	c.MainMailURL = ""
	c.IndependentMail = old.IndependentMail
	c.Hostname = strings.ToLower(strings.TrimSpace(c.Hostname))
	c.Domain = strings.ToLower(strings.TrimSpace(c.Domain))
	c.Origin = strings.TrimRight(c.Origin, "/")
	c.OIDCClaim = "sub"
	if c.EngineHostname == "" {
		c.EngineHostname = c.Hostname
	}
	if !domainPattern.MatchString(c.EngineHostname) {
		return errors.New("invalid SMTP hostname")
	}
	if !domainPattern.MatchString(c.Hostname) || !domainPattern.MatchString(c.Domain) || !validURL(c.Origin) {
		return errors.New("Enter valid mail domains and an HTTPS public URL (HTTP is allowed only on localhost)")
	}
	if old.Issuer != "" {
		c.Issuer = old.Issuer
		c.ClientID = old.ClientID
		c.ClientSecret = old.ClientSecret
		c.OwnerSubject = old.OwnerSubject
	}
	if c.Issuer != "" && (!validURL(c.Issuer) || c.ClientID == "" || c.ClientSecret == "") {
		return errors.New("MyAccounts requires its issuer URL, client ID and client secret")
	}
	if c.EnginePort == 0 {
		c.EnginePort = old.EnginePort
	}
	if c.SMTPPort == 0 {
		c.SMTPPort = old.SMTPPort
	}
	if c.EnginePort < 1024 || c.SMTPPort < 1024 || c.EnginePort > 65535 || c.SMTPPort > 65535 || c.EnginePort == a.port || c.SMTPPort == a.port || c.EnginePort == c.SMTPPort {
		return errors.New("Choose distinct unprivileged web, engine and local SMTP ports")
	}
	_, engineErr := os.Stat(filepath.Join(a.data, "engine.json"))
	resume := engineErr == nil
	if resume {
		var pending Config
		if e := platform.LoadSecret(filepath.Join(a.data, "pending.dpapi"), &pending); e != nil || pending.Domain != c.Domain || pending.Hostname != c.Hostname || pending.Issuer != c.Issuer || pending.ClientID != c.ClientID || pending.EnginePort != c.EnginePort || pending.SMTPPort != c.SMTPPort || (pending.EngineHostname != "" && pending.EngineHostname != c.EngineHostname) || pending.AutomaticHosting != c.AutomaticHosting {
			return errors.New("Incomplete setup belongs to different settings. Retain the existing data and inspect engine.log before migrating.")
		}
	}
	if a.sharedRoot != "" {
		return a.configureSharedMail(parent, c)
	}
	a.stopEngine()
	ports := []int{c.EnginePort, c.SMTPPort}
	if c.AutomaticHosting {
		ports = append(ports, 25)
	}
	for _, port := range ports {
		host := "127.0.0.1"
		if port == 25 {
			host = "0.0.0.0"
		}
		l, e := net.Listen("tcp4", fmt.Sprintf("%s:%d", host, port))
		if e != nil {
			return fmt.Errorf("Port %d is occupied", port)
		}
		l.Close()
	}
	credentials := map[string]string{}
	if e := platform.LoadSecret(filepath.Join(a.data, "engine.dpapi"), &credentials); e != nil && !os.IsNotExist(e) {
		return e
	}
	password := credentials["password"]
	if password == "" {
		password = platform.Token()
	}
	if e := platform.SaveSecret(filepath.Join(a.data, "engine.dpapi"), map[string]string{"password": password}); e != nil {
		return e
	}
	if e := platform.SaveSecret(filepath.Join(a.data, "pending.dpapi"), c); e != nil {
		return e
	}
	a.mu.Lock()
	a.cfg = c
	a.mu.Unlock()
	if e := a.startEngineMode(resume); e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:"+password))
	var defaults map[string]any
	var e error
	if !resume {
		for n := 0; n < 60; n++ {
			defaults, e = a.jmap(ctx, auth, "x:Bootstrap/get", map[string]any{})
			if e == nil {
				break
			}
			select {
			case <-ctx.Done():
				return errors.New("Stalwart bootstrap timed out; inspect protected engine.log")
			case <-time.After(500 * time.Millisecond):
			}
		}
		if e != nil {
			return e
		}
		list, _ := defaults["list"].([]any)
		if len(list) == 0 {
			return errors.New("Bootstrap settings unavailable")
		}
		object := list[0].(map[string]any)
		id, _ := object["id"].(string)
		patch := map[string]any{"serverHostname": c.EngineHostname, "defaultDomain": c.Domain, "requestTlsCertificate": false, "generateDkimKeys": !c.AutomaticHosting, "dataStore": map[string]any{"@type": "RocksDb", "path": filepath.ToSlash(filepath.Join(a.data, "store"))}, "tracer": map[string]any{"@type": "Stdout", "enable": true}, "dnsServer": map[string]any{"@type": "Manual"}}
		if c.Issuer != "" {
			patch["directory"] = map[string]any{"@type": "Oidc", "description": "MyAccounts", "issuerUrl": c.Issuer, "requireAudience": c.ClientID, "requireScopes": map[string]bool{}, "claimUsername": "sub", "usernameDomain": c.Domain, "claimName": "name", "claimGroups": nil}
		}
		if _, e = a.jmap(ctx, auth, "x:Bootstrap/set", map[string]any{"update": map[string]any{id: patch}}); e != nil {
			return e
		}
		// Bootstrap writes the persistent registry; reopen it in recovery mode before
		// adding listeners, so no upstream public listeners are ever started.
		a.stopEngine()
		if e = a.startEngineMode(true); e != nil {
			return e
		}
	}
	for n := 0; n < 60; n++ {
		_, e = a.jmap(ctx, auth, "x:NetworkListener/get", map[string]any{})
		if e == nil {
			break
		}
		select {
		case <-ctx.Done():
			return errors.New("Recovery startup timed out")
		case <-time.After(500 * time.Millisecond):
		}
	}
	if e != nil {
		return e
	}
	// Insert our own listeners before the first normal restart, suppressing upstream defaults.
	existing, e := a.jmap(ctx, auth, "x:NetworkListener/get", map[string]any{})
	if e != nil {
		return e
	}
	listeners := map[string]any{"http": map[string]any{"name": "workspace-http", "protocol": "http", "bind": map[string]bool{fmt.Sprintf("127.0.0.1:%d", c.EnginePort): true}, "useTls": false, "tlsImplicit": false}, "smtp": map[string]any{"name": "smtp", "protocol": "smtp", "bind": map[string]bool{fmt.Sprintf("127.0.0.1:%d", c.SMTPPort): true}, "useTls": false, "tlsImplicit": false}}
	if c.AutomaticHosting {
		listeners["internet-smtp"] = map[string]any{"name": "workspace-internet-smtp", "protocol": "smtp", "bind": map[string]bool{"0.0.0.0:25": true}, "useTls": true, "tlsImplicit": false}
	}
	updates := map[string]any{}
	if list, ok := existing["list"].([]any); ok {
		for _, item := range list {
			v := item.(map[string]any)
			id, _ := v["id"].(string)
			switch v["name"] {
			case "workspace-http":
				delete(listeners["http"].(map[string]any), "name")
				updates[id] = listeners["http"]
				delete(listeners, "http")
			case "smtp":
				delete(listeners["smtp"].(map[string]any), "name")
				updates[id] = listeners["smtp"]
				delete(listeners, "smtp")
			case "workspace-internet-smtp":
				if !c.AutomaticHosting {
					return errors.New("unexpected Internet mail listener")
				}
				delete(listeners["internet-smtp"].(map[string]any), "name")
				updates[id] = listeners["internet-smtp"]
				delete(listeners, "internet-smtp")
			default:
				return errors.New("Unexpected existing listener; review Stalwart before continuing")
			}
		}
	}
	_, e = a.jmap(ctx, auth, "x:NetworkListener/set", map[string]any{"create": listeners, "update": updates})
	if e != nil {
		return e
	}
	// Recovery mode does not create the engine's standard roles or authentication
	// defaults. All listeners are now explicitly restricted to this installation,
	// so start normally before connecting MyAccounts. Asking for a User role in
	// recovery mode leaves every fresh OIDC installation stuck before first boot.
	a.stopEngine()
	if e = a.startEngine(); e != nil {
		return e
	}
	if e = a.configureIdentity(ctx); e != nil {
		return e
	}
	c.EngineSchema = 2
	if a.engine != nil {
		a.stopEngine()
	}
	c.Configured = true
	if e = platform.SaveSecret(filepath.Join(a.data, "mail.dpapi"), c); e != nil {
		return e
	}
	a.mu.Lock()
	a.cfg = c
	a.mu.Unlock()
	if e = a.startEngine(); e != nil {
		c.Configured = false
		_ = platform.SaveSecret(filepath.Join(a.data, "mail.dpapi"), c)
		a.mu.Lock()
		a.cfg = c
		a.mu.Unlock()
		return e
	}
	return nil
}
