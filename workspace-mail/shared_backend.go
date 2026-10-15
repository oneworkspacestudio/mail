// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/fahimc/stalwart/workspace-mail/internal/platform"
	"github.com/oneworkspacestudio/calendar/shared"
	"os"
	"path/filepath"
	"time"
)

func (a *App) attachShared(parent context.Context) error {
	c, _, e := shared.Load(a.sharedRoot)
	if e != nil {
		return e
	}
	if _, e = shared.Install(a.sharedRoot, "mail", version, nil); e != nil {
		return e
	}
	self, _ := os.Executable()
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	if e = shared.Ensure(ctx, a.sharedRoot, filepath.Join(filepath.Dir(self), "WorkspaceServices.exe")); e != nil {
		return e
	}
	a.cfg.Issuer = c.Issuer
	a.cfg.ClientID = c.ClientID
	a.cfg.ClientSecret = c.ClientSecret
	a.cfg.Domain = c.Domain
	a.cfg.EnginePort = c.EnginePort
	a.cfg.Configured = true
	a.cfg.EngineSchema = 2
	a.cfg.AutomaticHosting = false
	return nil
}
func (a *App) configureSharedMail(parent context.Context, c Config) error {
	if c.Issuer == "" || c.ClientID == "" || c.ClientSecret == "" {
		return errors.New("MyAccounts is required for the shared Mail backend")
	}
	self, _ := os.Executable()
	dir := filepath.Dir(self)
	var manifest struct {
		SHA string `json:"engineSha256"`
	}
	raw, e := os.ReadFile(filepath.Join(dir, "engine.lock.json"))
	if e != nil {
		return errors.New("Install the shared service package beside Mail.exe")
	}
	if e = json.Unmarshal(raw, &manifest); e != nil {
		return e
	}
	initial := shared.Config{Issuer: c.Issuer, ClientID: c.ClientID, ClientSecret: c.ClientSecret, Domain: c.Domain, Hostname: c.Hostname, EngineHostname: c.EngineHostname, EnginePort: c.EnginePort, Version: shared.EngineVersion, EnginePath: filepath.Join(dir, "stalwart.exe"), EngineSHA256: manifest.SHA}
	if _, e = shared.Install(a.sharedRoot, "mail", version, &initial); e != nil {
		return e
	}
	a.cfg = c
	if e = a.attachShared(parent); e != nil {
		return e
	}
	return platform.SaveSecret(filepath.Join(a.data, "mail.dpapi"), a.cfg)
}
