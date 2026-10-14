// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"encoding/base64"
	"errors"
	"github.com/fahimc/stalwart/workspace-mail/internal/platform"
	"net/http"
	"path/filepath"
)

// Configure a TLS-only outgoing relay without sending a verification message.
func (a *App) delivery(w http.ResponseWriter, r *http.Request) {
	if !a.setupAllowed(w, r) {
		return
	}
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	a.mu.RLock()
	c := a.cfg
	a.mu.RUnlock()
	if !c.Configured {
		fail(w, errors.New("Configure Mail first"))
		return
	}
	var in struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Secret   string `json:"secret"`
	}
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	if !domainPattern.MatchString(in.Host) || (in.Port != 465 && in.Port != 587) || in.Username == "" || in.Secret == "" {
		fail(w, errors.New("Enter your relay hostname, port 465 (TLS) or 587 (STARTTLS), username and password"))
		return
	}
	var credentials map[string]string
	if e := platform.LoadSecret(filepath.Join(a.data, "engine.dpapi"), &credentials); e != nil {
		fail(w, e)
		return
	}
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:"+credentials["password"]))
	routes, e := a.jmap(r.Context(), auth, "x:MtaRoute/get", map[string]any{})
	if e != nil {
		fail(w, e)
		return
	}
	relay := map[string]any{"@type": "Relay", "name": "workspace-relay", "description": "One Workspace outgoing relay", "address": in.Host, "port": in.Port, "protocol": "smtp", "implicitTls": in.Port == 465, "allowInvalidCerts": false, "authUsername": in.Username, "authSecret": map[string]any{"@type": "Value", "secret": in.Secret}}
	existing := ""
	if list, ok := routes["list"].([]any); ok {
		for _, item := range list {
			v := item.(map[string]any)
			if v["name"] == "workspace-relay" {
				existing, _ = v["id"].(string)
			}
		}
	}
	args := map[string]any{"create": map[string]any{"relay": relay}}
	if existing != "" {
		args = map[string]any{"update": map[string]any{existing: relay}}
	}
	if _, e = a.jmap(r.Context(), auth, "x:MtaRoute/set", args); e != nil {
		fail(w, e)
		return
	}
	strategies, e := a.jmap(r.Context(), auth, "x:MtaOutboundStrategy/get", map[string]any{"ids": []string{"singleton"}})
	if e != nil {
		fail(w, e)
		return
	}
	list, _ := strategies["list"].([]any)
	id := ""
	if len(list) == 1 {
		id, _ = list[0].(map[string]any)["id"].(string)
	} else {
		id, e = a.singletonID(r.Context(), auth)
		if e != nil {
			fail(w, e)
			return
		}
	}
	if _, e = a.jmap(r.Context(), auth, "x:MtaOutboundStrategy/set", map[string]any{"update": map[string]any{id: map[string]any{"route/else": "'workspace-relay'"}}}); e != nil {
		fail(w, e)
		return
	}
	c.RelayHost = in.Host
	c.RelayPort = in.Port
	c.RelayUsername = in.Username
	c.RelaySecret = in.Secret
	if e = platform.SaveSecret(filepath.Join(a.data, "mail.dpapi"), c); e != nil {
		fail(w, e)
		return
	}
	a.mu.Lock()
	a.cfg = c
	a.restarting = true
	a.mu.Unlock()
	_ = a.engine.Process.Kill()
	<-a.engineDone
	a.mu.Lock()
	a.restarting = false
	a.mu.Unlock()
	if e = a.startEngine(); e != nil {
		fail(w, e)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "message": "TLS relay configured. No email was sent. Confirm the sending domain with your relay provider and publish its required DNS records."})
}
