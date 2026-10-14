// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/fahimc/stalwart/workspace-mail/internal/platform"
	"path/filepath"
	"time"
)

// The per-domain directory and the default Bearer-token directory are separate
// settings in Stalwart. Both must select this installation's MyAccounts client.
func (a *App) configureIdentity(ctx context.Context) error {
	a.mu.RLock()
	c := a.cfg
	a.mu.RUnlock()
	if c.Issuer == "" {
		return nil
	}
	var credentials map[string]string
	if e := platform.LoadSecret(filepath.Join(a.data, "engine.dpapi"), &credentials); e != nil {
		return e
	}
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:"+credentials["password"]))
	var directories map[string]any
	var e error
	for n := 0; n < 60; n++ {
		directories, e = a.jmap(ctx, auth, "x:Directory/get", map[string]any{})
		if e == nil {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	if e != nil {
		return e
	}
	id := ""
	if list, ok := directories["list"].([]any); ok {
		for _, item := range list {
			v := item.(map[string]any)
			if v["issuerUrl"] == c.Issuer {
				id, _ = v["id"].(string)
			}
		}
	}
	if id == "" {
		return errors.New("MyAccounts directory was not created")
	}
	settings, e := a.jmap(ctx, auth, "x:Authentication/get", map[string]any{"ids": []string{"singleton"}})
	if e != nil {
		return e
	}
	list, _ := settings["list"].([]any)
	settingID := ""
	if len(list) == 1 {
		settingID, _ = list[0].(map[string]any)["id"].(string)
	} else {
		settingID, e = a.singletonID(ctx, auth)
		if e != nil {
			return e
		}
	}
	_, e = a.jmap(ctx, auth, "x:Authentication/set", map[string]any{"update": map[string]any{settingID: map[string]any{"directoryId": id}}})
	return e
}
func (a *App) singletonID(ctx context.Context, auth string) (string, error) {
	// Stalwart's Id::singleton() (20080258862541) encodes as this literal.
	return "singleton", nil
}
