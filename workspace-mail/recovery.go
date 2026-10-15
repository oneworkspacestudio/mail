// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"errors"
	"github.com/fahimc/stalwart/workspace-mail/internal/platform"
	"net/http"
	"path/filepath"
	"time"
)

// Repair uses private recovery credentials only for configuration. All mailbox
// operations continue to use the signed-in user's own access token.
func (a *App) repairIdentity(w http.ResponseWriter, r *http.Request) {
	if !a.setupAllowed(w, r) {
		return
	}
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	a.mu.RLock()
	configured := a.cfg.Configured
	a.mu.RUnlock()
	if !configured {
		fail(w, errors.New("Finish Mail setup before repairing mailbox access"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	if err := a.configureIdentity(ctx); err != nil {
		fail(w, err)
		return
	}
	a.mu.Lock()
	a.cfg.EngineSchema = 2
	a.identityError = ""
	err := platform.SaveSecret(filepath.Join(a.data, "mail.dpapi"), a.cfg)
	a.mu.Unlock()
	if err != nil {
		fail(w, err)
		return
	}
	// Drop native cached permissions before reopening the user's mailbox.
	auth, err := a.engineAdmin()
	if err != nil {
		fail(w, err)
		return
	}
	a.stopEngine()
	if err = a.startEngine(); err != nil {
		fail(w, err)
		return
	}
	for attempt := 0; attempt < 60; attempt++ {
		_, err = a.jmap(ctx, auth, "x:Authentication/get", map[string]any{"ids": []string{"singleton"}})
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			fail(w, ctx.Err())
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "message": "Mailbox permissions repaired. Open Mail again; sign in again if your previous session has expired."})
}

type MailAccessError struct{ Status int }

func (e *MailAccessError) Error() string {
	switch e.Status {
	case 401:
		return "Mail could not verify your MyAccounts session. Sign out of Mail and sign in again. If it repeats, ask the Workspace owner to repair mailbox access in Mail settings."
	case 403:
		return "MyAccounts signed you in, but Mail denied mailbox access. The Workspace owner can use Repair mailbox access below. This is separate from Internet mail DNS setup."
	default:
		return "The Mail engine is unavailable. Retry in a moment; if it continues, open Workspace startup progress and check Mail."
	}
}

func engineAccessError(status int) error { return &MailAccessError{Status: status} }
