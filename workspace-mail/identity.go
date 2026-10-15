// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/fahimc/stalwart/workspace-mail/internal/platform"
	"path/filepath"
	"strings"
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
			if v["issuerUrl"] == c.Issuer && v["requireAudience"] == c.ClientID && v["claimUsername"] == "sub" && v["usernameDomain"] == c.Domain {
				if id != "" {
					return errors.New("More than one matching MyAccounts directory exists; review Mail identity settings")
				}
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
	patch := map[string]any{"directoryId": id}
	// Restore only a missing ordinary-user role link, preserving custom policy.
	var roleLinks map[string]any
	if len(list) == 1 {
		roleLinks, _ = list[0].(map[string]any)["defaultUserRoleIds"].(map[string]any)
	}
	if len(roleLinks) == 0 {
		roles, err := a.jmap(ctx, auth, "x:Role/get", map[string]any{})
		if err != nil {
			return err
		}
		items, _ := roles["list"].([]any)
		userRole := ""
		for _, item := range items {
			role, ok := item.(map[string]any)
			if !ok || role["description"] != "User" || role["memberTenantId"] != nil {
				continue
			}
			enabled, _ := role["enabledPermissions"].(map[string]any)
			disabled, _ := role["disabledPermissions"].(map[string]any)
			inherited, _ := role["roleIds"].(map[string]any)
			if enabled["authenticate"] != true || enabled["jmapMailboxGet"] != true || len(inherited) != 0 || len(disabled) != 0 {
				continue
			}
			safe := true
			for permission := range enabled {
				if !ordinaryUserPermission(permission) {
					safe = false
				}
			}
			if !safe {
				continue
			}
			if userRole != "" {
				return errors.New("Multiple default User roles exist; the Workspace owner must review permissions")
			}
			userRole, _ = role["id"].(string)
		}
		if userRole == "" {
			return errors.New("Mail's ordinary User role is missing or changed. Restore the standard User role before repairing sign-in")
		}
		patch["defaultUserRoleIds"] = map[string]bool{userRole: true}
	}
	_, e = a.jmap(ctx, auth, "x:Authentication/set", map[string]any{"update": map[string]any{settingID: patch}})
	if e == nil {
		check, err := a.jmap(ctx, auth, "x:Authentication/get", map[string]any{"ids": []string{"singleton"}})
		if err != nil {
			return err
		}
		items, _ := check["list"].([]any)
		if len(items) != 1 {
			return errors.New("Mail identity settings were not saved")
		}
		links, _ := items[0].(map[string]any)["defaultUserRoleIds"].(map[string]any)
		if len(links) == 0 {
			return errors.New("Mail's default user permissions are still missing")
		}
	}
	return e
}

// Matches the ordinary-user defaults in the pinned engine's permissions.rs.
// Some sys* permissions are scoped to the user's own settings and credentials.
func ordinaryUserPermission(p string) bool {
	if p == "authenticate" || p == "authenticateWithAlias" || p == "interactAi" {
		return true
	}
	for _, prefix := range []string{"jmap", "imap", "pop3", "calendar", "email", "dav", "sieve", "sysMaskedEmail", "sysArchivedItem", "sysAccountSettings", "sysPublicKey", "sysAccountPassword", "sysApiKey", "sysAppPassword"} {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return strings.HasPrefix(p, "sysSpamTrainingSample") && !strings.Contains(p, "Create")
}
func (a *App) singletonID(ctx context.Context, auth string) (string, error) {
	// Stalwart's Id::singleton() (20080258862541) encodes as this literal.
	return "singleton", nil
}
