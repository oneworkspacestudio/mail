// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGatewayEndpoint(t *testing.T) {
	peer := net.ParseIP("192.168.4.1")
	for _, tt := range []struct {
		url  string
		want bool
	}{{"http://192.168.4.1:1900/ipc", true}, {"http://192.168.4.1/igd.xml", true}, {"http://127.0.0.1/control", false}, {"http://169.254.169.254/latest/", false}, {"http://192.168.4.2/control", false}, {"http://example.com/control", false}, {"https://192.168.4.1/control", false}, {"http://user:secret@192.168.4.1/control", false}, {"http://192.168.4.1/control#fragment", false}} {
		if got := gatewayEndpoint(tt.url, peer); got != tt.want {
			t.Errorf("endpoint %q=%v", tt.url, got)
		}
	}
}

func TestRouterSMTPMapping(t *testing.T) {
	for _, tt := range []struct {
		name, existing     string
		wantAdd, wantError bool
	}{
		{"free", "", true, false}, {"owned-renewal", "owned", true, false}, {"other-server", "other", false, true}, {"same-target-manual", "manual", false, false}, {"disabled", "disabled", false, true}, {"unknown-query-failure", "unknown", false, true}, {"readback-mismatch", "mismatch", true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			added := false
			queries := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				payload, _ := io.ReadAll(r.Body)
				decoder := xml.NewDecoder(strings.NewReader(string(payload)))
				for {
					_, err := decoder.Token()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Errorf("invalid SOAP XML: %v", err)
						http.Error(w, "invalid XML", 400)
						return
					}
				}
				action := r.Header.Get("SOAPAction")
				w.Header().Set("Content-Type", "text/xml")
				if strings.Contains(action, "#AddPortMapping") {
					added = true
					for _, expected := range []string{"<NewExternalPort>25</NewExternalPort>", "<NewInternalPort>25</NewInternalPort>", "<NewInternalClient>192.168.4.37</NewInternalClient>", "<NewLeaseDuration>1800</NewLeaseDuration>", smtpMappingDescription} {
						if !strings.Contains(string(payload), expected) {
							t.Errorf("missing scoped mapping field %q", expected)
						}
					}
					_, _ = io.WriteString(w, `<Envelope><Body><AddPortMappingResponse/></Body></Envelope>`)
					return
				}
				queries++
				if (tt.existing == "" || tt.existing == "mismatch") && !added {
					w.WriteHeader(500)
					_, _ = io.WriteString(w, `<Envelope><Body><Fault><errorCode>714</errorCode></Fault></Body></Envelope>`)
					return
				}
				if tt.existing == "unknown" {
					w.WriteHeader(500)
					_, _ = io.WriteString(w, `<Envelope><Body><Fault><errorCode>501</errorCode></Fault></Body></Envelope>`)
					return
				}
				client, description, enabled := "192.168.4.37", smtpMappingDescription, "1"
				if tt.existing == "other" || tt.existing == "mismatch" {
					client = "192.168.4.99"
				}
				if tt.existing == "manual" {
					description = "Existing administrator rule"
				}
				if tt.existing == "disabled" {
					enabled = "0"
				}
				_, _ = io.WriteString(w, "<Envelope><Body><GetSpecificPortMappingEntryResponse><NewInternalClient>"+client+"</NewInternalClient><NewInternalPort>25</NewInternalPort><NewEnabled>"+enabled+"</NewEnabled><NewPortMappingDescription>"+description+"</NewPortMappingDescription></GetSpecificPortMappingEntryResponse></Body></Envelope>")
			}))
			defer server.Close()
			gateway := mailGateway{Service: "urn:schemas-upnp-org:service:WANIPConnection:1", Control: server.URL}
			_, err := gateway.ensureSMTP(context.Background(), "192.168.4.37")
			if (err != nil) != tt.wantError {
				t.Fatalf("error=%v, expected error %v", err, tt.wantError)
			}
			if added != tt.wantAdd {
				t.Fatalf("mapping mutation=%v, expected%v", added, tt.wantAdd)
			}
			if tt.wantAdd && queries != 2 {
				t.Fatal("mapping was not read back")
			}
		})
	}
}

func TestIncomingProbeState(t *testing.T) {
	for _, tt := range []struct{ name, data, want string }{
		{"reachable", `{"one":[{"time":0.1,"address":"81.107.35.22"}],"two":[{"error":"timeout"}]}`, "reachable"},
		{"pending", `{"one":null,"two":[{"error":"timeout"}]}`, "unverified"},
		{"blocked", `{"one":[{"error":"timeout"}],"two":[{"error":"refused"}]}`, "attention"},
		{"malformed", `{"one":{"time":0}}`, "unverified"},
		{"private-result", `{"one":[{"time":0,"address":"192.168.4.37"}]}`, "unverified"},
		{"shared-nat", `{"one":[{"time":0,"address":"100.64.0.1"}]}`, "unverified"},
		{"empty", `{}`, "unverified"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var results map[string]json.RawMessage
			if err := json.Unmarshal([]byte(tt.data), &results); err != nil {
				t.Fatal(err)
			}
			if got := incomingProbeState(results); got != tt.want {
				t.Fatalf("got %s want %s", got, tt.want)
			}
		})
	}
	for _, id := range []string{"", "../internal", "query?", "bad/id", strings.Repeat("a", 81)} {
		if validProbeID(id) {
			t.Fatalf("unsafe probe ID accepted %q", id)
		}
	}
	if !validProbeID("511d9f68k353") {
		t.Fatal("valid probe ID rejected")
	}
}
