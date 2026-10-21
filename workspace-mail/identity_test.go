package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fahimc/stalwart/workspace-mail/internal/platform"
)

func TestNativeFreshWorkspaceMailIdentity(t *testing.T) {
	for _, scenario := range []string{"fresh", "retry before defaults", "retry after defaults"} {
		t.Run(scenario, func(t *testing.T) { testNativeWorkspaceMailIdentity(t, scenario) })
	}
}

type identityRoundTrip func(*http.Request) (*http.Response, error)

func (f identityRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testNativeWorkspaceMailIdentity(t *testing.T, scenario string) {
	if os.Getenv("WORKSPACE_MAIL_NATIVE_TEST") != "1" {
		t.Skip("native fixture requires opt-in")
	}
	data := t.TempDir()
	if err := platform.ProtectDir(data); err != nil {
		t.Fatal(err)
	}
	job, err := platform.NewJob()
	if err != nil {
		t.Fatal(err)
	}
	defer job.Close()
	port := func() int {
		l, e := net.Listen("tcp4", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		p := l.Addr().(*net.TCPAddr).Port
		l.Close()
		return p
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		base := "http://" + r.Host
		if r.URL.Path == "/keys" {
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "fixture", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()), "e": "AQAB"}}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": base, "authorization_endpoint": base + "/authorize", "token_endpoint": base + "/token", "userinfo_endpoint": base + "/userinfo", "jwks_uri": base + "/keys", "response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"RS256"}})
	}))
	defer idp.Close()
	a := &App{data: data, port: port(), job: job, client: &http.Client{Timeout: 45 * time.Second}, cfg: Config{EnginePort: port(), SMTPPort: port(), Issuer: idp.URL, ClientID: "fixture-mail", ClientSecret: "fixture-secret", OwnerSubject: "fixture-owner"}}
	defer a.stopEngine()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	c := a.cfg
	c.Hostname = "mail.example.test"
	c.Domain = "example.test"
	c.Origin = "http://localhost:20696"
	interruptedRoles := map[string]bool{}
	if scenario != "fresh" {
		injected := false
		a.client.Transport = identityRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.GetBody != nil && !injected {
				body, e := r.GetBody()
				if e != nil {
					return nil, e
				}
				var request struct {
					MethodCalls [][]any `json:"methodCalls"`
				}
				_ = json.NewDecoder(body).Decode(&request)
				body.Close()
				if len(request.MethodCalls) > 0 && request.MethodCalls[0][0] == "x:NetworkListener/set" && scenario == "retry before defaults" {
					resp, e := http.DefaultTransport.RoundTrip(r)
					if e != nil {
						return resp, e
					}
					_, _ = io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					injected = true
					return nil, errors.New("synthetic interruption before native defaults")
				}
				if len(request.MethodCalls) > 0 && request.MethodCalls[0][0] == "x:Authentication/get" && scenario == "retry after defaults" {
					injected = true
					return nil, errors.New("synthetic interrupted setup")
				}
			}
			return http.DefaultTransport.RoundTrip(r)
		})
		if err = a.configureMail(ctx, c); err == nil || !injected {
			t.Fatal("interrupted setup was not exercised")
		}
		a.client.Transport = nil
		auth, _ := a.engineAdmin()
		roles, e := a.jmap(ctx, auth, "x:Role/get", map[string]any{})
		if e != nil {
			t.Fatal(e)
		}
		for _, role := range objectList(roles) {
			id, _ := role["id"].(string)
			interruptedRoles[id] = true
		}
		if scenario == "retry before defaults" && len(interruptedRoles) != 0 {
			t.Fatal("fixture already initialized roles")
		}
		a.stopEngine()
	}
	if err = a.configureMail(ctx, c); err != nil {
		raw, _ := os.ReadFile(filepath.Join(data, "engine.log"))
		lines := strings.Split(string(raw), "\n")
		if len(lines) > 20 {
			lines = lines[len(lines)-20:]
		}
		t.Log(strings.Join(lines, "\n"))
		auth, _ := a.engineAdmin()
		roles, e := a.jmap(ctx, auth, "x:Role/get", map[string]any{})
		if e == nil {
			t.Logf("Fresh role definitions: %+v", roles["list"])
		}
		t.Fatal(err)
	}
	auth, err := a.engineAdmin()
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	for n := 0; n < 60; n++ {
		settings, err = a.jmap(ctx, auth, "x:Authentication/get", map[string]any{"ids": []string{"singleton"}})
		if err == nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	links, _ := objectList(settings)[0]["defaultUserRoleIds"].(map[string]any)
	if len(links) != 1 {
		t.Fatalf("expected native default user role, got %v", links)
	}
	roles, err := a.jmap(ctx, auth, "x:Role/get", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if len(objectList(roles)) != 4 {
		t.Fatal("native role defaults were not initialized")
	}
	for _, role := range objectList(roles) {
		id, _ := role["id"].(string)
		if len(interruptedRoles) > 0 && !interruptedRoles[id] {
			t.Fatal("retry replaced existing native roles")
		}
		if links[id] != true {
			continue
		}
		if role["description"] != "User" {
			t.Fatal("default user gained an administrator role")
		}
		permissions, _ := role["enabledPermissions"].(map[string]any)
		for p := range permissions {
			if !ordinaryUserPermission(p) {
				t.Fatalf("unexpected user permission %s", p)
			}
		}
	}
	users := []MailboxUser{{Subject: "fixture-owner", Username: "owner", Name: "Owner"}, {Subject: "fixture-user", Username: "jane", Name: "Jane"}}
	addresses, err := a.ensureMailboxes(ctx, users)
	if err != nil {
		t.Fatal(err)
	}
	if addresses["fixture-owner"] != "admin@example.test" || addresses["fixture-user"] != "jane@example.test" {
		t.Fatal("mailbox addresses were not provisioned")
	}
	sign := func(subject, audience string) string {
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"fixture"}`))
		claims, _ := json.Marshal(map[string]any{"iss": idp.URL, "aud": audience, "sub": subject, "name": "Fixture user", "exp": time.Now().Add(10 * time.Minute).Unix(), "iat": time.Now().Unix()})
		unsigned := header + "." + base64.RawURLEncoding.EncodeToString(claims)
		sum := sha256.Sum256([]byte(unsigned))
		signature, e := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
		if e != nil {
			t.Fatal(e)
		}
		return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
	}
	for _, subject := range []string{"fixture-owner", "fixture-user"} {
		userAuth := "Bearer " + sign(subject, "fixture-mail")
		resp, e := a.engineRequest(ctx, "GET", "/jmap/session", userAuth, nil, "")
		if e != nil {
			t.Fatal(e)
		}
		var session map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&session)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("mailbox sign-in returned %d", resp.StatusCode)
		}
		accounts, _ := session["accounts"].(map[string]any)
		if len(accounts) != 1 {
			t.Fatal("mailbox access was not scoped to the signed-in user")
		}
		for id := range accounts {
			if _, e = a.jmap(ctx, userAuth, "Mailbox/get", map[string]any{"accountId": id}); e != nil {
				t.Fatal(e)
			}
		}
		if _, e = a.jmap(ctx, userAuth, "x:Role/get", map[string]any{}); e == nil {
			t.Fatal("ordinary mailbox token gained role administration")
		}
	}
	resp, err := a.engineRequest(ctx, "GET", "/jmap/session", "Bearer "+sign("fixture-user", "another-app"), nil, "")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatal("wrong audience was accepted")
	}
	a.stopEngine()
	if err = a.startEngine(); err != nil {
		t.Fatal(err)
	}
	if err = a.configureIdentity(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := a.ensureMailboxes(ctx, users)
	if err != nil || got["fixture-owner"] != addresses["fixture-owner"] || got["fixture-user"] != addresses["fixture-user"] {
		t.Fatalf("restart changed mailboxes: %v", err)
	}
}
