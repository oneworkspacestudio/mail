// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPrivateRouterHops(t *testing.T) {
	got := parsePrivateMailHops("Tracing route to 1.1.1.1\n1 1 ms 192.168.4.1\n2 3 ms 192.168.1.1\n3 9 ms 81.1.2.3\n192.168.1.1 999.2.3.4 127.0.0.1")
	if len(got) != 2 || got[0].String() != "192.168.4.1" || got[1].String() != "192.168.1.1" {
		t.Fatalf("unexpected hops: %v", got)
	}
}

func TestUpstreamRuleOwnership(t *testing.T) {
	for _, tt := range []struct {
		name, description, target, port, enabled string
		legacy, want                             bool
	}{
		{"ours", "One Workspace Mail abc SMTP", "192.168.1.120", "25", "1", false, true},
		{"legacy-migration", smtpMappingDescription, "192.168.1.120", "25", "1", true, true},
		{"legacy-without-local-proof", smtpMappingDescription, "192.168.1.120", "25", "1", false, false},
		{"another-install", "One Workspace Mail other SMTP", "192.168.1.120", "25", "1", true, false},
		{"unrelated", "Other mail server", "192.168.1.120", "25", "1", true, false},
		{"wrong-subnet", "One Workspace Mail abc SMTP", "192.168.9.120", "25", "1", false, false},
		{"router-itself", "One Workspace Mail abc SMTP", "192.168.1.1", "25", "1", false, false},
		{"wrong-port", "One Workspace Mail abc SMTP", "192.168.1.120", "587", "1", false, false},
		{"disabled", "One Workspace Mail abc SMTP", "192.168.1.120", "25", "0", false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			description, added := tt.description, false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if strings.Contains(r.Header.Get("SOAPAction"), "#AddPortMapping") {
					added = true
					description = "One Workspace Mail abc SMTP"
					if !strings.Contains(string(body), "<NewInternalClient>192.168.1.120</NewInternalClient>") {
						t.Error("changed downstream target")
					}
					fmt.Fprint(w, "<Envelope/>")
					return
				}
				fmt.Fprintf(w, "<Envelope><NewInternalClient>%s</NewInternalClient><NewInternalPort>%s</NewInternalPort><NewEnabled>%s</NewEnabled><NewPortMappingDescription>%s</NewPortMappingDescription></Envelope>", tt.target, tt.port, tt.enabled, description)
			}))
			defer server.Close()
			_, err := renewUpstreamSMTP(context.Background(), mailGateway{"urn:schemas-upnp-org:service:WANIPConnection:1", server.URL}, net.ParseIP("192.168.1.1"), "One Workspace Mail abc SMTP", tt.legacy)
			if (err == nil) != tt.want || added != tt.want {
				t.Fatalf("err=%v mutation=%v want=%v", err, added, tt.want)
			}
		})
	}
}

func TestRecoveryPreservesChangedRule(t *testing.T) {
	for _, changed := range []bool{false, true} {
		var actions []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			action := r.Header.Get("SOAPAction")
			actions = append(actions, action)
			if strings.Contains(action, "#GetSpecificPortMappingEntry") {
				description := "One Workspace Mail abc SMTP"
				if changed {
					description = "Other administrator"
				}
				fmt.Fprintf(w, "<Envelope><NewInternalClient>192.168.1.120</NewInternalClient><NewInternalPort>25</NewInternalPort><NewEnabled>1</NewEnabled><NewPortMappingDescription>%s</NewPortMappingDescription></Envelope>", description)
			} else {
				fmt.Fprint(w, "<Envelope/>")
			}
		}))
		err := refreshOwnedMailRoutes(context.Background(), []smtpRoute{{mailGateway{"urn:schemas-upnp-org:service:WANIPConnection:1", server.URL}, "192.168.1.120", "One Workspace Mail abc SMTP"}})
		server.Close()
		if changed && (err == nil || len(actions) != 1) {
			t.Fatalf("changed rule was not preserved: %v %v", err, actions)
		}
		if !changed && (err != nil || len(actions) != 5 || !strings.Contains(actions[1], "#DeletePortMapping") || !strings.Contains(actions[3], "#AddPortMapping")) {
			t.Fatalf("recovery not verified: %v %v", err, actions)
		}
	}
}

func TestLegacyRouterDescriptionMigration(t *testing.T) {
	const desired = "Workspace Mail 123456abcdef"
	if len(desired) > 31 {
		t.Fatal("router description too long")
	}
	description, deleted := smtpMappingDescription, false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.Header.Get("SOAPAction")
		switch {
		case strings.Contains(action, "#DeletePortMapping"):
			deleted = true
		case strings.Contains(action, "#AddPortMapping"):
			if deleted {
				description = desired
			}
		case strings.Contains(action, "#GetSpecificPortMappingEntry"):
			fmt.Fprintf(w, "<Envelope><NewInternalClient>192.168.4.37</NewInternalClient><NewInternalPort>25</NewInternalPort><NewEnabled>1</NewEnabled><NewPortMappingDescription>%s</NewPortMappingDescription></Envelope>", description)
			return
		}
		fmt.Fprint(w, "<Envelope/>")
	}))
	defer server.Close()
	_, err := (mailGateway{"urn:schemas-upnp-org:service:WANIPConnection:1", server.URL}).ensureSMTP(context.Background(), "192.168.4.37", desired)
	if err != nil || !deleted || description != desired {
		t.Fatalf("legacy rule not migrated: %v", err)
	}
}
