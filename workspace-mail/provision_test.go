// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	netsmtp "net/smtp"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fahimc/stalwart/workspace-mail/internal/platform"
)

func TestMailboxLocal(t *testing.T) {
	for _, tt := range []struct{ input, want string }{{"Jane.Smith", "jane.smith"}, {"john_doe", "john.doe"}, {" somebody@example.net ", "somebody"}, {"...", "user"}, {"你好", "user"}, {"Jane / Doe", "jane.doe"}} {
		if got := mailboxLocal(tt.input); got != tt.want {
			t.Errorf("mailboxLocal(%q)=%q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestProvisionEndpointAuthorization(t *testing.T) {
	t.Setenv("WORKSPACE_MAIL_PROVISION_TOKEN", strings.Repeat("a", 64))
	a := &App{port: 20696}
	for _, tt := range []struct{ name, token, host, remote, forward string }{{"missing", "", "localhost:20696", "127.0.0.1:1234", ""}, {"wrong", strings.Repeat("b", 64), "localhost:20696", "127.0.0.1:1234", ""}, {"public-host", strings.Repeat("a", 64), "workspace.example.test", "127.0.0.1:1234", ""}, {"remote", strings.Repeat("a", 64), "localhost:20696", "192.0.2.5:1234", ""}, {"forwarded", strings.Repeat("a", 64), "localhost:20696", "127.0.0.1:1234", "192.0.2.5"}} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://"+tt.host+"/api/mailboxes/provision", strings.NewReader(`{"users":[]}`))
			r.RemoteAddr = tt.remote
			r.Header.Set("Authorization", "Bearer "+tt.token)
			r.Header.Set("X-Forwarded-For", tt.forward)
			w := httptest.NewRecorder()
			a.provisionMailboxes(w, r)
			if w.Code != 403 {
				t.Fatalf("status=%d, want403", w.Code)
			}
		})
	}
}

// Explicit opt-in: uses only a new private NTFS test directory and pinned engine,
// sends no mail, and stops its native child before TempDir cleanup.
func TestNativeMailboxProvisioning(t *testing.T) {
	if os.Getenv("WORKSPACE_MAIL_NATIVE_TEST") != "1" {
		t.Skip("native fixture requires explicit opt-in")
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
	web, engine, smtp := port(), port(), port()
	a := &App{data: data, port: web, job: job, client: &http.Client{Timeout: 10 * time.Second}, cfg: Config{EnginePort: engine, SMTPPort: smtp}}
	defer a.stopEngine()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	c := Config{Hostname: "mail.example.test", Domain: "example.test", Origin: "http://localhost:20696", EnginePort: engine, SMTPPort: smtp}
	if err = a.configureMail(ctx, c); err != nil {
		t.Fatal(err)
	}
	a.cfg.Issuer = "http://localhost:20691"
	a.cfg.ClientID = "fixture-mail"
	a.cfg.OwnerSubject = "owner-id"
	auth, err := a.engineAdmin()
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 60; n++ {
		if _, err = a.jmap(ctx, auth, "x:Domain/get", map[string]any{}); err == nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	users := []MailboxUser{{Subject: "new-user", Username: "Jane.Smith", Name: "Jane"}, {Subject: "owner-id", Username: "owner", Name: "Owner"}, {Subject: "collision-user", Username: "Jane Smith", Name: "Another Jane"}, {Subject: "reserved-user", Username: "admin", Name: "Another admin"}}
	// Internal bootstrap owns admin@example.test. Never take that account over.
	if _, err = a.ensureMailboxes(ctx, users); err == nil || !strings.Contains(err.Error(), "already owned") {
		t.Fatalf("expected existing admin collision, got %v", err)
	}
	domains, err := a.jmap(ctx, auth, "x:Domain/get", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	var directory any
	for _, domain := range objectList(domains) {
		if domain["name"] == "example.test" {
			directory = domain["directoryId"]
		}
	}
	_, err = a.jmap(ctx, auth, "x:Domain/set", map[string]any{"create": map[string]any{"fixture": map[string]any{"name": "users.example.test", "directoryId": directory}}})
	if err != nil {
		t.Fatal(err)
	}
	a.cfg.Domain = "users.example.test"
	// Upgrade an existing ID-based mailbox without changing its account ID or aliases.
	domains, err = a.jmap(ctx, auth, "x:Domain/get", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	var domainID any
	for _, domain := range objectList(domains) {
		if domain["name"] == a.cfg.Domain {
			domainID = domain["id"]
		}
	}
	legacy, err := a.jmap(ctx, auth, "x:Account/set", map[string]any{"create": map[string]any{"legacy": map[string]any{"@type": "User", "name": "owner-id", "domainId": domainID, "roles": map[string]any{"@type": "User"}, "credentials": map[string]any{}, "aliases": map[string]any{"7": map[string]any{"name": "legacy-owner", "domainId": domainID, "enabled": true}}}}})
	if err != nil {
		t.Fatal(err)
	}
	created, _ := legacy["created"].(map[string]any)
	oldOwner, _ := created["legacy"].(map[string]any)
	addresses, err := a.ensureMailboxes(ctx, users)
	if err != nil {
		t.Fatal(err)
	}
	for subject, want := range map[string]string{"owner-id": "admin@users.example.test", "new-user": "jane.smith@users.example.test", "collision-user": "jane.smith.2@users.example.test", "reserved-user": "admin.2@users.example.test"} {
		if addresses[subject] != want {
			t.Fatalf("address for %s=%s, want%s", subject, addresses[subject], want)
		}
	}
	// Recipient routing works before any Mail login. QUIT before DATA sends no mail.
	// Model inbound port25 on the fixture's unprivileged loopback-only listener.
	// Production submission authentication is never changed by this test.
	_, err = a.jmap(ctx, auth, "x:MtaStageAuth/set", map[string]any{"update": map[string]any{"singleton": map[string]any{"require/else": "false"}}})
	if err != nil {
		t.Fatal(err)
	}
	a.stopEngine()
	if err = a.startEngine(); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 60; n++ {
		if _, err = a.jmap(ctx, auth, "x:Domain/get", map[string]any{}); err == nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", smtp), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	client, err := netsmtp.NewClient(conn, "localhost")
	if err != nil {
		conn.Close()
		t.Fatal(err)
	}
	defer client.Close()
	if err = client.Hello("localhost"); err != nil {
		t.Fatal(err)
	}
	if err = client.Mail("probe@example.test"); err != nil {
		t.Fatal(err)
	}
	if err = client.Rcpt(addresses["new-user"]); err != nil {
		t.Fatal(err)
	}
	if err = client.Quit(); err != nil {
		t.Fatal(err)
	}
	users[0].Username = "renamed-user"
	again, err := a.ensureMailboxes(ctx, users)
	if err != nil {
		t.Fatal(err)
	}
	if again["new-user"] != addresses["new-user"] {
		t.Fatal("renaming changed assigned address")
	}
	state, err := a.loadMailboxBindings(a.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if state.Users["owner-id"].AccountID != oldOwner["id"] {
		t.Fatal("changed the existing owner's mailbox ID")
	}
	owner, err := a.jmap(ctx, auth, "x:Account/get", map[string]any{"ids": []string{state.Users["owner-id"].AccountID}})
	if err != nil {
		t.Fatal(err)
	}
	ownerList := objectList(owner)
	aliases, _ := ownerList[0]["aliases"].(map[string]any)
	legacyAlias := false
	for _, raw := range aliases {
		alias, _ := raw.(map[string]any)
		if alias["name"] == "legacy-owner" && alias["enabled"] == true {
			legacyAlias = true
		}
	}
	if !legacyAlias {
		t.Fatal("removed an existing alias")
	}
	objects, err := a.jmap(ctx, auth, "x:Account/get", map[string]any{"properties": []string{"id", "roles", "name"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(objectList(objects)) != 5 {
		t.Fatal("reconciliation created duplicate accounts")
	}
	for _, obj := range objectList(objects) {
		if obj["name"] == "admin" {
			continue
		}
		roles, _ := obj["roles"].(map[string]any)
		if roles["@type"] != "User" {
			t.Fatal("provisioned a privileged mailbox")
		}
	}
}
