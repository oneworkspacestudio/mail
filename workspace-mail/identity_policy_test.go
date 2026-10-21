package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/fahimc/stalwart/workspace-mail/internal/platform"
)

func TestIdentityPreservesExistingCustomRolePolicy(t *testing.T) {
	data := t.TempDir()
	if err := platform.SaveSecret(filepath.Join(data, "engine.dpapi"), map[string]string{"password": "fixture"}); err != nil {
		t.Fatal(err)
	}
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MethodCalls [][]json.RawMessage `json:"methodCalls"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var method string
		_ = json.Unmarshal(req.MethodCalls[0][0], &method)
		var result any
		switch method {
		case "x:Directory/get":
			result = map[string]any{"list": []any{map[string]any{"id": "directory", "issuerUrl": "https://accounts.example.test", "requireAudience": "mail", "claimUsername": "sub", "usernameDomain": "example.test"}}}
		case "x:Authentication/get":
			result = map[string]any{"list": []any{map[string]any{"id": "singleton", "directoryId": "directory", "defaultUserRoleIds": map[string]bool{"custom-role": true}}}}
		case "x:Authentication/set":
			var args struct {
				Update map[string]map[string]any `json:"update"`
			}
			_ = json.Unmarshal(req.MethodCalls[0][1], &args)
			patch := args.Update["singleton"]
			if len(patch) != 1 || patch["directoryId"] != "directory" {
				t.Error("custom permission policy changed")
			}
			writes++
			result = map[string]any{"updated": map[string]any{"singleton": nil}}
		default:
			t.Errorf("unexpected policy mutation/read: %s", method)
			result = map[string]any{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"methodResponses": []any{[]any{method, result, "m"}}})
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	a := &App{data: data, client: server.Client(), cfg: Config{EnginePort: port, Issuer: "https://accounts.example.test", ClientID: "mail", Domain: "example.test"}}
	if err := a.configureIdentity(context.Background()); err != nil {
		t.Fatal(err)
	}
	if writes != 1 {
		t.Fatal("directory connection was not saved")
	}
}

func TestIdentityWarningKeepsTechnicalDetailsOutOfPublicStatus(t *testing.T) {
	technical := "Mail's ordinary User role is missing or changed. Restore the standard User role before repairing sign-in"
	err := &mailIdentitySetupError{cause: fmtIdentityError(technical)}
	if !strings.Contains(err.Error(), "Reconnect Mail") || strings.Contains(err.Error(), "User role") {
		t.Fatal("technical repair instructions exposed")
	}
	if err.Unwrap().Error() != technical {
		t.Fatal("support diagnostic lost")
	}
	a := &App{identityError: technical}
	w := httptest.NewRecorder()
	a.status(w, httptest.NewRequest("GET", "http://localhost/api/status", nil))
	if strings.Contains(w.Body.String(), "User role") || !strings.Contains(w.Body.String(), "Reconnect Mail") {
		t.Fatal(w.Body.String())
	}
}

type fmtIdentityError string

func (e fmtIdentityError) Error() string { return string(e) }
