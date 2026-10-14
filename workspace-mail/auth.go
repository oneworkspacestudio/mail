// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/fahimc/stalwart/workspace-mail/internal/platform"
	"golang.org/x/oauth2"
	"net/http"
	"strings"
	"time"
)

type Login struct {
	Nonce, Verifier string
	Expires         time.Time
}

func (a *App) oidcConfig(ctx context.Context) (*oidc.Provider, oauth2.Config, error) {
	a.mu.RLock()
	c := a.cfg
	a.mu.RUnlock()
	if c.Issuer == "" || c.ClientID == "" || c.ClientSecret == "" {
		return nil, oauth2.Config{}, errors.New("Configure MyAccounts in setup first")
	}
	p, e := oidc.NewProvider(ctx, c.Issuer)
	if e != nil {
		return nil, oauth2.Config{}, errors.New("MyAccounts discovery is unavailable")
	}
	return p, oauth2.Config{ClientID: c.ClientID, ClientSecret: c.ClientSecret, RedirectURL: c.Origin + a.base + "/auth/callback", Endpoint: p.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "profile", "email", "offline_access"}}, nil
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	_, o, e := a.oidcConfig(ctx)
	if e != nil {
		fail(w, e)
		return
	}
	state := platform.Token()
	nonce := platform.Token()
	v := platform.Token()
	sum := sha256.Sum256([]byte(v))
	a.mu.Lock()
	for k, s := range a.pending {
		if s.Expires.Before(time.Now()) {
			delete(a.pending, k)
		}
	}
	if len(a.pending) > 500 {
		a.mu.Unlock()
		http.Error(w, "Please retry later", 429)
		return
	}
	a.pending[state] = Login{nonce, v, time.Now().Add(5 * time.Minute)}
	a.mu.Unlock()
	a.cookie(w, "mail_login", state, 300)
	http.Redirect(w, r, o.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.SetAuthURLParam("code_challenge", base64.RawURLEncoding.EncodeToString(sum[:])), oauth2.SetAuthURLParam("code_challenge_method", "S256")), 302)
}
func (a *App) callback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	cookie, e := r.Cookie("mail_login")
	if e != nil || cookie.Value != state || state == "" {
		http.Error(w, "Login state rejected", 403)
		return
	}
	a.mu.Lock()
	s, ok := a.pending[state]
	delete(a.pending, state)
	a.mu.Unlock()
	a.cookie(w, "mail_login", "", -1)
	if !ok || time.Now().After(s.Expires) {
		http.Error(w, "Login expired", 403)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	p, o, e := a.oidcConfig(ctx)
	if e != nil {
		fail(w, e)
		return
	}
	token, e := o.Exchange(ctx, r.URL.Query().Get("code"), oauth2.SetAuthURLParam("code_verifier", s.Verifier))
	if e != nil {
		fail(w, errors.New("MyAccounts did not complete sign-in"))
		return
	}
	id, _ := token.Extra("id_token").(string)
	verified, e := p.Verifier(&oidc.Config{ClientID: o.ClientID}).Verify(ctx, id)
	if e != nil || verified.Nonce != s.Nonce {
		http.Error(w, "Identity token rejected", 403)
		return
	}
	var claims struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if e = verified.Claims(&claims); e != nil {
		http.Error(w, "Identity claims rejected", 403)
		return
	}
	session := &Session{Auth: "Bearer " + token.AccessToken, Subject: verified.Subject, Name: claims.Name, Email: claims.Email, CSRF: platform.Token(), Expires: time.Now().Add(8 * time.Hour), TokenExpiry: token.Expiry, RefreshToken: token.RefreshToken}
	a.newSession(w, session)
	http.Redirect(w, r, a.base+"/", 302)
}
func (a *App) cookie(w http.ResponseWriter, name, value string, max int) {
	a.mu.RLock()
	secure := strings.HasPrefix(a.cfg.Origin, "https:")
	a.mu.RUnlock()
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: a.base + "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: max})
}
func (a *App) newSession(w http.ResponseWriter, s *Session) {
	key := platform.Token()
	a.mu.Lock()
	for k, v := range a.sessions {
		if v.Expires.Before(time.Now()) {
			delete(a.sessions, k)
		}
	}
	if len(a.sessions) >= 1000 {
		a.mu.Unlock()
		http.Error(w, "Too many sessions", 429)
		return
	}
	a.sessions[key] = s
	a.mu.Unlock()
	a.cookie(w, "mail_session", key, 8*3600)
}
func (a *App) user(w http.ResponseWriter, r *http.Request) (*Session, bool) {
	c, e := r.Cookie("mail_session")
	if e != nil {
		http.Error(w, "Sign in required", 401)
		return nil, false
	}
	a.mu.RLock()
	s, ok := a.sessions[c.Value]
	a.mu.RUnlock()
	if !ok || s.Expires.Before(time.Now()) {
		http.Error(w, "Sign in required", 401)
		return nil, false
	}
	if r.Method != "GET" && r.Method != "HEAD" && r.Header.Get("X-CSRF-Token") != s.CSRF {
		http.Error(w, "Refresh the page before trying again", 403)
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.HasPrefix(s.Auth, "Bearer ") && !s.TokenExpiry.IsZero() && time.Now().After(s.TokenExpiry.Add(-30*time.Second)) {
		if s.RefreshToken == "" {
			http.Error(w, "Sign in again", 401)
			return nil, false
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		_, o, e := a.oidcConfig(ctx)
		if e != nil {
			http.Error(w, "Sign in again", 401)
			return nil, false
		}
		t, e := o.TokenSource(ctx, &oauth2.Token{RefreshToken: s.RefreshToken}).Token()
		if e != nil {
			http.Error(w, "Sign in again", 401)
			return nil, false
		}
		s.Auth = "Bearer " + t.AccessToken
		s.TokenExpiry = t.Expiry
		if t.RefreshToken != "" {
			s.RefreshToken = t.RefreshToken
		}
	}
	return s, true
}
func (a *App) sessionInfo(w http.ResponseWriter, r *http.Request) {
	s, ok := a.user(w, r)
	if !ok {
		return
	}
	a.mu.RLock()
	c := a.cfg
	a.mu.RUnlock()
	writeJSON(w, map[string]any{"name": s.Name, "email": s.Email, "subject": s.Subject, "csrf": s.CSRF, "base": a.base, "owner": s.Subject == c.OwnerSubject, "domain": c.Domain, "version": version})
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.user(w, r); !ok {
		return
	}
	if c, e := r.Cookie("mail_session"); e == nil {
		a.mu.Lock()
		delete(a.sessions, c.Value)
		a.mu.Unlock()
	}
	a.cookie(w, "mail_session", "", -1)
	writeJSON(w, map[string]bool{"ok": true})
}
func (a *App) passwordLogin(w http.ResponseWriter, r *http.Request) {
	// Optional standalone login uses ordinary Stalwart credentials, never recovery admin.
	a.mu.RLock()
	oidcEnabled := a.cfg.Issuer != ""
	a.mu.RUnlock()
	if oidcEnabled {
		http.Error(w, "Use MyAccounts", 403)
		return
	}
	var in struct{ Email, Password string }
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	if !strings.Contains(in.Email, "@") || in.Password == "" {
		http.Error(w, "Use your mailbox email and password", 400)
		return
	}
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(in.Email+":"+in.Password))
	session, e := a.engineRequest(r.Context(), "GET", "/jmap/session", auth, nil, "")
	if e != nil {
		http.Error(w, "Mailbox sign-in failed", 401)
		return
	}
	defer session.Body.Close()
	if session.StatusCode != 200 {
		http.Error(w, "Mailbox sign-in failed", 401)
		return
	}
	var details struct {
		PrimaryAccounts map[string]string `json:"primaryAccounts"`
	}
	_ = json.NewDecoder(session.Body).Decode(&details)
	if details.PrimaryAccounts["urn:ietf:params:jmap:mail"] == "" {
		http.Error(w, "No mailbox access", 403)
		return
	}
	a.newSession(w, &Session{Auth: auth, Subject: in.Email, Email: in.Email, Name: strings.Split(in.Email, "@")[0], CSRF: platform.Token(), Expires: time.Now().Add(8 * time.Hour)})
	writeJSON(w, map[string]bool{"ok": true})
}
