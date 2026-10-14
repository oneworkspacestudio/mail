// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
)

var allowedMethods = map[string]bool{"Core/echo": true, "Mailbox/get": true, "Mailbox/query": true, "Mailbox/set": true, "Email/get": true, "Email/query": true, "Email/changes": true, "Email/queryChanges": true, "Email/set": true, "Thread/get": true, "Identity/get": true, "EmailSubmission/set": true, "EmailSubmission/get": true, "SearchSnippet/get": true, "Quota/get": true, "VacationResponse/get": true, "VacationResponse/set": true}

func (s *Session) auth() string { s.mu.Lock(); defer s.mu.Unlock(); return s.Auth }
func (a *App) engineRequest(ctx context.Context, method, path, auth string, body io.Reader, contentType string) (*http.Response, error) {
	a.mu.RLock()
	port := a.cfg.EnginePort
	a.mu.RUnlock()
	req, e := http.NewRequestWithContext(ctx, method, fmt.Sprintf("http://127.0.0.1:%d", port)+path, body)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Authorization", auth)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return a.client.Do(req)
}
func (a *App) jmap(ctx context.Context, auth, method string, args any) (map[string]any, error) {
	caps := []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:mail", "urn:ietf:params:jmap:submission"}
	if strings.HasPrefix(method, "x:") {
		caps = append(caps, "urn:stalwart:jmap")
	}
	b, _ := json.Marshal(map[string]any{"using": caps, "methodCalls": []any{[]any{method, args, "m"}}})
	resp, e := a.engineRequest(ctx, "POST", "/jmap", auth, bytes.NewReader(b), "application/json")
	if e != nil {
		return nil, errors.New("Stalwart is not reachable; check engine.log")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Stalwart rejected the request (HTTP %d)", resp.StatusCode)
	}
	var out struct {
		MethodResponses [][]json.RawMessage `json:"methodResponses"`
	}
	if e = json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&out); e != nil {
		return nil, e
	}
	if len(out.MethodResponses) != 1 || len(out.MethodResponses[0]) < 2 {
		return nil, errors.New("Unexpected Stalwart response")
	}
	var name string
	_ = json.Unmarshal(out.MethodResponses[0][0], &name)
	var result map[string]any
	if e = json.Unmarshal(out.MethodResponses[0][1], &result); e != nil {
		return nil, e
	}
	if name == "error" {
		return nil, fmt.Errorf("%s: %v (%v)", method, result["type"], result["description"])
	}
	for _, k := range []string{"notCreated", "notUpdated", "notDestroyed"} {
		if v, ok := result[k].(map[string]any); ok && len(v) > 0 {
			return nil, fmt.Errorf("%s: %v", method, v)
		}
	}
	return result, nil
}
func (a *App) proxyJMAP(w http.ResponseWriter, r *http.Request) {
	s, ok := a.user(w, r)
	if !ok {
		return
	}
	if r.URL.Query().Get("session") == "1" {
		resp, e := a.engineRequest(r.Context(), "GET", "/jmap/session", s.auth(), nil, "")
		if e != nil {
			fail(w, errors.New("Stalwart is starting or is unavailable"))
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, io.LimitReader(resp.Body, 2<<20))
		return
	}
	var in struct {
		Method string         `json:"method"`
		Args   map[string]any `json:"args"`
	}
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	if !allowedMethods[in.Method] {
		http.Error(w, "Method unavailable", 403)
		return
	}
	result, e := a.jmap(r.Context(), s.auth(), in.Method, in.Args)
	if e != nil {
		fail(w, e)
		return
	}
	writeJSON(w, result)
}
func (a *App) upload(w http.ResponseWriter, r *http.Request) {
	s, ok := a.user(w, r)
	if !ok {
		return
	}
	account := r.URL.Query().Get("accountId")
	if account == "" || strings.ContainsAny(account, "/\\") {
		http.Error(w, "Missing account", 400)
		return
	}
	body := http.MaxBytesReader(w, r.Body, 25<<20)
	resp, e := a.engineRequest(r.Context(), "POST", "/jmap/upload/"+url.PathEscape(account)+"/", s.auth(), body, r.Header.Get("Content-Type"))
	if e != nil {
		fail(w, errors.New("Upload failed"))
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 1<<20))
}
func (a *App) download(w http.ResponseWriter, r *http.Request) {
	s, ok := a.user(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	account, blob := q.Get("accountId"), q.Get("blobId")
	if account == "" || blob == "" || strings.ContainsAny(account+blob, "/\\") {
		http.Error(w, "Invalid attachment", 400)
		return
	}
	name := q.Get("name")
	if name == "" {
		name = "attachment"
	}
	resp, e := a.engineRequest(r.Context(), "GET", "/jmap/download/"+url.PathEscape(account)+"/"+url.PathEscape(blob)+"/"+url.PathEscape(name), s.auth(), nil, "")
	if e != nil {
		fail(w, errors.New("Download failed"))
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 50<<20))
}
