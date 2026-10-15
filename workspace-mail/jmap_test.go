// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"strings"
	"testing"
)

func TestParseJMAPResponse(t *testing.T) {
	tests := []struct {
		name, method, body, wantError string
		warning                       bool
	}{
		{"single", "Mailbox/get", `{"methodResponses":[["Mailbox/get",{"list":[]},"m"]]}`, "", false},
		{"submission-and-sent", "EmailSubmission/set", `{"methodResponses":[["EmailSubmission/set",{"created":{"send":{"id":"s1"}}},"m"],["Email/set",{"updated":{"e1":null}},"m"]]}`, "", false},
		{"implicit-empty-call-id", "EmailSubmission/set", `{"methodResponses":[["EmailSubmission/set",{"created":{"send":{"id":"s1"}}},"m"],["Email/set",{"updated":{"e1":null}},""]]}`, "", false},
		{"primary-after-effect", "EmailSubmission/set", `{"methodResponses":[["Email/set",{"updated":{}},"m"],["EmailSubmission/set",{"created":{"send":{"id":"s1"}}},"m"]]}`, "", false},
		{"sent-update-failed", "EmailSubmission/set", `{"methodResponses":[["EmailSubmission/set",{"created":{"send":{"id":"s1"}}},"m"],["Email/set",{"notUpdated":{"e1":{"type":"forbidden"}}},"m"]]}`, "", true},
		{"implicit-method-error", "EmailSubmission/set", `{"methodResponses":[["EmailSubmission/set",{"created":{"send":{"id":"s1"}}},"m"],["error",{"type":"serverFail"},"m"]]}`, "", true},
		{"method-error", "EmailSubmission/set", `{"methodResponses":[["error",{"type":"forbidden","description":"Not allowed"},"m"]]}`, "forbidden", false},
		{"send-rejected", "EmailSubmission/set", `{"methodResponses":[["EmailSubmission/set",{"notCreated":{"send":{"type":"invalidRecipients"}}},"m"]]}`, "invalidRecipients", false},
		{"update-rejected", "Email/set", `{"methodResponses":[["Email/set",{"notUpdated":{"e1":{"type":"forbidden"}}},"m"]]}`, "forbidden", false},
		{"wrong-call", "EmailSubmission/set", `{"methodResponses":[["EmailSubmission/set",{"created":{}},"other"]]}`, "duplicate", false},
		{"missing-primary", "EmailSubmission/set", `{"methodResponses":[["Email/set",{"updated":{}},"m"]]}`, "duplicate", false},
		{"duplicate-primary", "EmailSubmission/set", `{"methodResponses":[["EmailSubmission/set",{},"m"],["EmailSubmission/set",{},"m"]]}`, "duplicate", false},
		{"malformed-tuple", "EmailSubmission/set", `{"methodResponses":[["EmailSubmission/set",{}]]}`, "duplicate", false},
		{"null-result", "EmailSubmission/set", `{"methodResponses":[["EmailSubmission/set",null,"m"]]}`, "duplicate", false},
		{"html", "EmailSubmission/set", `<html>upstream unavailable</html>`, "duplicate", false},
		{"missing-results", "Mailbox/get", `{}`, "Refresh Mail", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseJMAPResponse(tt.method, strings.NewReader(tt.body))
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("wanted error containing %q, got %v", tt.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			_, warning := result["workspaceWarnings"]
			if warning != tt.warning {
				t.Fatalf("warning = %v, want %v", warning, tt.warning)
			}
			if tt.method == "EmailSubmission/set" && result["created"] == nil {
				t.Fatal("lost the successful submission result")
			}
		})
	}
}
