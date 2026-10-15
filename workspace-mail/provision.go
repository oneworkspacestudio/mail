// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/fahimc/stalwart/workspace-mail/internal/platform"
)

type MailboxUser struct {
	Subject  string `json:"subject"`
	Username string `json:"username"`
	Name     string `json:"displayName"`
}
type MailboxBinding struct {
	AccountID string `json:"accountId"`
	Address   string `json:"address"`
}
type MailboxBindings struct {
	Issuer   string                    `json:"issuer"`
	ClientID string                    `json:"clientId"`
	Domain   string                    `json:"domain"`
	Users    map[string]MailboxBinding `json:"users"`
}

var mailboxSubject = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var mailboxLocalInvalid = regexp.MustCompile(`[^a-z0-9]+`)
var reservedMailboxNames = map[string]bool{"admin": true, "postmaster": true, "abuse": true, "root": true, "mailer-daemon": true, "noreply": true}

func mailboxLocal(username string) string {
	username = strings.SplitN(strings.ToLower(strings.TrimSpace(username)), "@", 2)[0]
	name := strings.Trim(mailboxLocalInvalid.ReplaceAllString(username, "."), ".")
	if len(name) > 48 {
		name = strings.TrimRight(name[:48], ".")
	}
	if name == "" {
		name = "user"
	}
	return name
}

func (a *App) loadMailboxBindings(c Config) (MailboxBindings, error) {
	state := MailboxBindings{Issuer: c.Issuer, ClientID: c.ClientID, Domain: c.Domain, Users: map[string]MailboxBinding{}}
	err := platform.LoadSecret(filepath.Join(a.data, "mailboxes.dpapi"), &state)
	if err != nil && !os.IsNotExist(err) {
		return state, errors.New("Mailbox address assignments could not be read")
	}
	if state.Issuer != c.Issuer || state.ClientID != c.ClientID || state.Domain != c.Domain {
		return state, errors.New("Mailbox assignments belong to a different identity connection or domain; review them before migration")
	}
	if state.Users == nil {
		state.Users = map[string]MailboxBinding{}
	}
	return state, nil
}

func (a *App) mailboxAddress(subject string) string {
	if a.sharedRoot != "" {
		return subject + "@" + a.cfg.Domain
	}
	a.provisionMu.Lock()
	defer a.provisionMu.Unlock()
	a.mu.RLock()
	c := a.cfg
	a.mu.RUnlock()
	state, err := a.loadMailboxBindings(c)
	if err != nil {
		return ""
	}
	return state.Users[subject].Address
}

// Only the local Workspace supervisor can provision accounts. User-token JMAP
// remains the exclusive path for reading or modifying mailbox messages.
func (a *App) provisionMailboxes(w http.ResponseWriter, r *http.Request) {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(host)
	secret := os.Getenv("WORKSPACE_MAIL_PROVISION_TOKEN")
	provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if len(secret) < 32 || subtle.ConstantTimeCompare([]byte(secret), []byte(provided)) != 1 || ip == nil || !ip.IsLoopback() || r.Header.Get("X-Forwarded-For") != "" || (r.Host != fmt.Sprintf("localhost:%d", a.port) && r.Host != fmt.Sprintf("127.0.0.1:%d", a.port)) {
		http.Error(w, "Local mailbox provisioning authorization required", http.StatusForbidden)
		return
	}
	var in struct {
		Users []MailboxUser `json:"users"`
	}
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	if len(in.Users) > 200 {
		http.Error(w, "Provision at most 200 users per batch", 400)
		return
	}
	addresses, err := a.ensureMailboxes(r.Context(), in.Users)
	if err != nil {
		a.mu.Lock()
		a.provisionError = err.Error()
		a.mu.Unlock()
		fail(w, err)
		return
	}
	a.mu.Lock()
	a.provisionError = ""
	a.mu.Unlock()
	writeJSON(w, map[string]any{"mailboxes": addresses})
}

func (a *App) ensureMailboxes(ctx context.Context, users []MailboxUser) (map[string]string, error) {
	a.provisionMu.Lock()
	defer a.provisionMu.Unlock()
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	a.mu.RLock()
	c := a.cfg
	a.mu.RUnlock()
	if !c.Configured || c.Issuer == "" {
		return nil, errors.New("Finish the MyAccounts Mail connection before provisioning mailboxes")
	}
	state, err := a.loadMailboxBindings(c)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, u := range users {
		if !mailboxSubject.MatchString(u.Subject) || len(u.Username) > 255 || u.Username == "" || len(u.Name) > 255 || seen[u.Subject] {
			return nil, errors.New("Invalid or duplicate MyAccounts user")
		}
		seen[u.Subject] = true
	}
	// Reserve the owner's address first regardless of directory sort order.
	sort.SliceStable(users, func(i, j int) bool { return users[i].Subject == c.OwnerSubject && users[j].Subject != c.OwnerSubject })
	auth, err := a.engineAdmin()
	if err != nil {
		return nil, err
	}
	domains, err := a.jmap(ctx, auth, "x:Domain/get", map[string]any{"properties": []string{"id", "name"}})
	if err != nil {
		return nil, err
	}
	domainID := ""
	for _, item := range objectList(domains) {
		if item["name"] == c.Domain {
			domainID, _ = item["id"].(string)
		}
	}
	if domainID == "" {
		return nil, errors.New("Mail's configured domain is missing")
	}
	accounts, err := a.jmap(ctx, auth, "x:Account/get", map[string]any{"properties": []string{"id", "@type", "name", "domainId", "aliases", "externalId"}})
	if err != nil {
		return nil, err
	}
	bySubject := map[string]map[string]any{}
	occupied := map[string]string{}
	for _, account := range objectList(accounts) {
		id, _ := account["id"].(string)
		if account["domainId"] == domainID {
			name, _ := account["name"].(string)
			occupied[strings.ToLower(name)] = id
			if account["@type"] == "User" {
				bySubject[name] = account
			}
		}
		if aliases, ok := account["aliases"].(map[string]any); ok {
			for _, raw := range aliases {
				alias, ok := raw.(map[string]any)
				if ok && alias["domainId"] == domainID {
					name, _ := alias["name"].(string)
					occupied[strings.ToLower(name)] = id
				}
			}
		}
	}
	// Historical assignments are never recycled when an IdP user is removed.
	for _, binding := range state.Users {
		local := strings.TrimSuffix(binding.Address, "@"+c.Domain)
		if _, ok := occupied[local]; !ok {
			occupied[local] = binding.AccountID
		}
	}
	result := map[string]string{}
	for _, u := range users {
		account := bySubject[u.Subject]
		id, _ := account["id"].(string)
		binding, assigned := state.Users[u.Subject]
		if assigned && (binding.AccountID != id || account == nil) {
			return nil, errors.New("A provisioned mailbox is missing or its identity changed; restore it before retrying")
		}
		local := ""
		if assigned {
			local = strings.TrimSuffix(binding.Address, "@"+c.Domain)
		} else if u.Subject == c.OwnerSubject {
			local = "admin"
		} else {
			local = mailboxLocal(u.Username)
		}
		base := local
		for n := 1; ; n++ {
			owner, taken := occupied[local]
			reserved := !assigned && u.Subject != c.OwnerSubject && reservedMailboxNames[local]
			if !reserved && (!taken || (id != "" && owner == id)) {
				break
			}
			if assigned || u.Subject == c.OwnerSubject {
				return nil, errors.New("The assigned administrator/mailbox address is already owned by another account. Choose or migrate that address explicitly")
			}
			if n > 10000 {
				return nil, errors.New("No available mailbox address could be allocated")
			}
			local = fmt.Sprintf("%s.%d", base, n+1)
		}
		aliases, _ := account["aliases"].(map[string]any)
		if aliases == nil {
			aliases = map[string]any{}
		}
		found := false
		for _, raw := range aliases {
			alias, ok := raw.(map[string]any)
			if ok && alias["name"] == local && alias["domainId"] == domainID {
				if alias["enabled"] != true {
					return nil, errors.New("The mailbox's managed address was disabled; an administrator must review it")
				}
				found = true
			}
		}
		if assigned && found {
			result[u.Subject] = binding.Address
			continue
		}
		if !found {
			next := 0
			for key := range aliases {
				n, e := strconv.Atoi(key)
				if e != nil {
					return nil, errors.New("The native alias list has an invalid identifier")
				}
				if n >= next {
					next = n + 1
				}
			}
			aliases[strconv.Itoa(next)] = map[string]any{"name": local, "domainId": domainID, "enabled": true, "description": "Workspace mail address"}
		}
		if account == nil {
			created, err := a.jmap(ctx, auth, "x:Account/set", map[string]any{"create": map[string]any{"mailbox": map[string]any{"@type": "User", "name": u.Subject, "domainId": domainID, "roles": map[string]any{"@type": "User"}, "credentials": map[string]any{}, "aliases": aliases, "description": u.Name}}})
			if err != nil {
				return nil, errors.New("Could not create a MyAccounts mailbox: " + err.Error())
			}
			objects, _ := created["created"].(map[string]any)
			obj, _ := objects["mailbox"].(map[string]any)
			id, _ = obj["id"].(string)
			if id == "" {
				return nil, errors.New("The mail engine did not confirm mailbox creation")
			}
		} else if !found {
			if _, err = a.jmap(ctx, auth, "x:Account/set", map[string]any{"update": map[string]any{id: map[string]any{"aliases": aliases}}}); err != nil {
				return nil, errors.New("Could not assign the mailbox address: " + err.Error())
			}
		}
		// Read back the actual native account before persisting or displaying it.
		check, err := a.jmap(ctx, auth, "x:Account/get", map[string]any{"ids": []string{id}, "properties": []string{"id", "name", "domainId", "aliases"}})
		if err != nil {
			return nil, err
		}
		confirmed := false
		for _, obj := range objectList(check) {
			if obj["name"] != u.Subject || obj["domainId"] != domainID {
				continue
			}
			items, _ := obj["aliases"].(map[string]any)
			for _, raw := range items {
				alias, ok := raw.(map[string]any)
				if ok && alias["name"] == local && alias["domainId"] == domainID && alias["enabled"] == true {
					confirmed = true
				}
			}
		}
		if !confirmed {
			return nil, errors.New("Mailbox address creation was not confirmed; retry synchronization")
		}
		address := local + "@" + c.Domain
		state.Users[u.Subject] = MailboxBinding{AccountID: id, Address: address}
		if err = platform.SaveSecret(filepath.Join(a.data, "mailboxes.dpapi"), state); err != nil {
			return nil, errors.New("The mailbox exists but its address assignment could not be saved; retry synchronization")
		}
		occupied[local] = id
		result[u.Subject] = address
	}
	return result, nil
}

func objectList(response map[string]any) []map[string]any {
	var out []map[string]any
	items, _ := response["list"].([]any)
	for _, item := range items {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, obj)
		}
	}
	return out
}
