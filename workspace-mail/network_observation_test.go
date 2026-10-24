// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fahimc/stalwart/workspace-mail/internal/platform"
)

func TestSMTPProbeObservation(t *testing.T) {
	now := time.Now()
	line := `2026-10-24T10:00:00Z INFO SMTP connection started (smtp.connection-start) listenerId = "workspace-internet-smtp", localPort = 25, remoteIp = 8.8.8.8, remotePort = 42000` + "\n"
	nodes := map[string][]string{"node": {"xx", "Country", "City", "8.8.8.8"}}
	results := map[string]json.RawMessage{"node": json.RawMessage(`[{"time":0.1,"address":"1.1.1.1"}]`)}
	targets := []net.IP{net.ParseIP("1.1.1.1")}
	for _, tt := range []struct {
		name, line string
		since      time.Time
		nodes      map[string][]string
		results    map[string]json.RawMessage
		targets    []net.IP
		want       bool
	}{
		{"matched", line, now, nodes, results, targets, true},
		{"old", line, now.Add(time.Hour), nodes, results, targets, false},
		{"wrong-receiver", "", now, nodes, results, targets, false},
		{"unrelated-scanner", strings.ReplaceAll(line, "8.8.8.8", "9.9.9.9"), now, nodes, results, targets, false},
		{"LAN-test", strings.ReplaceAll(line, "8.8.8.8", "192.168.4.30"), now, nodes, results, targets, false},
		{"wrong-listener", strings.ReplaceAll(line, "workspace-internet-smtp", "smtp"), now, nodes, results, targets, false},
		{"wrong-port", strings.ReplaceAll(line, "localPort = 25,", "localPort = 2525,"), now, nodes, results, targets, false},
		{"not-event", "untrusted " + line, now, nodes, results, targets, false},
		{"wrong-target", line, now, nodes, results, []net.IP{net.ParseIP("9.9.9.9")}, false},
		{"failed-node", line, now, nodes, map[string]json.RawMessage{"node": json.RawMessage(`[{"error":"Connection timed out"}]`)}, targets, false},
		{"missing-nodes", line, now, nil, results, targets, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			o := smtpObservation{}
			var out bytes.Buffer
			log := engineLog{out: &out, observe: o.observe}
			// Exercise real chunk assembly in the stdout bridge.
			b := []byte(tt.line)
			mid := len(b) / 2
			_, _ = log.Write(b[:mid])
			_, _ = log.Write(b[mid:])
			if got := o.confirms(tt.since, tt.nodes, tt.results, tt.targets); got != tt.want {
				t.Fatalf("confirmed=%v want %v", got, tt.want)
			}
		})
	}
	o := smtpObservation{}
	for i := 0; i < 1000; i++ {
		o.observe(line)
	}
	if len(o.connections) != 256 {
		t.Fatal("unbounded connection cache")
	}
}

func TestVerifiedIncomingHelp(t *testing.T) {
	h := incomingHelp("verified", &inboundRouteIssue{Code: "upstream_router", Router: "192.168.1.1"})
	if h.Code != "verified" || h.RouterURL != "" || strings.Contains(h.Summary, "inspect") {
		t.Fatalf("stale repair guidance: %+v", h)
	}
}

func TestIncomingReceiptEvidence(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	line := now.Format(time.RFC3339) + ` INFO Queued message for delivery (queue.message-queued) listenerId = "workspace-internet-smtp", localPort = 25, remoteIp = 8.8.8.8, remotePort = 40000, queueId = 123, from = "sender@example.test", to = ["person@example.test"], size = 1000`
	for _, tt := range []struct {
		name, line, domain, state, code string
		want                            bool
	}{
		{"accepted", line, "example.test", "unverified", "upstream_router", true},
		{"public-port", line, "example.test", "reachable", "router_permission", true},
		{"wrong-domain", line, "other.test", "unverified", "upstream_router", false},
		{"private", strings.ReplaceAll(line, "8.8.8.8", "192.168.1.20"), "example.test", "unverified", "upstream_router", false},
		{"outgoing", strings.ReplaceAll(line, "workspace-internet-smtp", "smtp"), "example.test", "unverified", "upstream_router", false},
		{"wrong-port", strings.ReplaceAll(line, "localPort = 25,", "localPort = 2525,"), "example.test", "unverified", "upstream_router", false},
		{"rejected", strings.ReplaceAll(line, "queue.message-queued", "smtp.rcpt-to"), "example.test", "unverified", "upstream_router", false},
		{"expired", strings.Replace(line, now.Format(time.RFC3339), now.Add(-3*time.Hour).Format(time.RFC3339), 1), "example.test", "unverified", "upstream_router", false},
		{"future", strings.Replace(line, now.Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339), 1), "example.test", "unverified", "upstream_router", false},
		{"blocked-now", line, "example.test", "attention", "upstream_router", false},
		{"other-receiver", line, "example.test", "unverified", "other_mail_server", false},
		{"dns-mismatch", line, "example.test", "unverified", "dns_pending", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := App{}
			a.smtpObservation.observe(tt.line)
			state, message, help, ok := a.incomingReceiptStatus(tt.state, tt.domain, inboundHelp{Code: tt.code, RouterURL: "http://192.168.1.1/"})
			if ok != tt.want {
				t.Fatalf("accepted=%v want %v", ok, tt.want)
			}
			if ok && (state != "receiving" || help.RouterURL != "" || strings.Contains(message, "person@") || strings.Contains(message, "sender@")) {
				t.Fatal("stale action or private address in receipt status")
			}
		})
	}
	// Private native metadata survives restart; old public probe observations do
	// not. A bounded suffix read never loads arbitrary mailbox content.
	p := filepath.Join(t.TempDir(), "engine.log")
	if err := os.WriteFile(p, []byte(line+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	a := App{}
	a.smtpObservation.restoreReceipts(p)
	if _, _, _, ok := a.incomingReceiptStatus("unverified", "example.test", inboundHelp{Code: "unverified"}); !ok {
		t.Fatal("receipt lost on restart")
	}
	if len(a.smtpObservation.connections) != 0 {
		t.Fatal("historical connections restored as live probe proof")
	}
}

func TestNativeSMTPObservation(t *testing.T) {
	if os.Getenv("WORKSPACE_MAIL_NATIVE_TEST") != "1" {
		t.Skip("requires isolated native fixture opt-in")
	}
	fixtureRoot := os.Getenv("WORKSPACE_MAIL_NATIVE_ROOT")
	if fixtureRoot == "" {
		t.Fatal("set WORKSPACE_MAIL_NATIVE_ROOT to an isolated NTFS scratch directory")
	}
	data, err := os.MkdirTemp(fixtureRoot, "smtp-observation-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(data); err != nil {
			t.Error(err)
		}
	})
	if err := platform.ProtectDir(data); err != nil {
		t.Fatalf("isolated fixture %s: %v", data, err)
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
		defer l.Close()
		return l.Addr().(*net.TCPAddr).Port
	}
	a := &App{data: data, port: port(), job: job, client: &http.Client{Timeout: 30 * time.Second}}
	defer a.stopEngine()
	c := Config{Hostname: "mail.example.test", Domain: "example.test", EngineHostname: "smtp.example.test", Origin: "http://localhost:20696", EnginePort: port(), SMTPPort: port()}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err = a.configureMail(ctx, c); err != nil {
		t.Fatal(err)
	}
	if err = a.ensureSMTPObservation(ctx); err != nil {
		t.Fatal(err)
	}
	if err = a.ensureSMTPObservation(ctx); err != nil {
		t.Fatalf("non-idempotent configuration: %v", err)
	}
	conn, err := net.DialTimeout("tcp4", fmt.Sprintf("127.0.0.1:%d", c.SMTPPort), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	b := make([]byte, 1024)
	_, _ = conn.Read(b)
	conn.Close()
	for n := 0; n < 40; n++ {
		log, _ := os.ReadFile(filepath.Join(data, "engine.log"))
		for _, line := range strings.Split(string(log), "\n") {
			if smtpStart.MatchString(line) {
				// Native fixture binds loopback on an ephemeral port. Verify the exact
				// event format; external/public matching is covered above.
				if !strings.Contains(line, "remoteIp = 127.0.0.1") || !strings.Contains(line, fmt.Sprintf("localPort = %d", c.SMTPPort)) {
					continue
				}
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("native engine did not expose the narrow connection-start event after reload")
}
