// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fahimc/stalwart/workspace-mail/internal/platform"
)

type mailDomainPlan struct {
	PreviousDomain string   `json:"previousDomain"`
	Domain         string   `json:"domain"`
	SMTPHostname   string   `json:"smtpHostname"`
	Unavailable    []string `json:"unavailable"`
}
type mailDomainChange struct {
	PreviousDomain string `json:"previousDomain"`
	Domain         string `json:"domain"`
	SMTPHostname   string `json:"smtpHostname"`
	DomainID       string `json:"domainId"`
	DirectoryID    string `json:"directoryId"`
	Zone           string `json:"zone"`
	Issuer         string `json:"issuer"`
	ClientID       string `json:"clientId"`
}

func (a *App) pendingMailDomain() *mailDomainChange {
	var change mailDomainChange
	if platform.LoadSecret(filepath.Join(a.data, "mail-domain-change.dpapi"), &change) != nil {
		return nil
	}
	// Only domain choices are sent to the UI; no native IDs or credentials.
	return &mailDomainChange{Domain: change.Domain, PreviousDomain: change.PreviousDomain, SMTPHostname: change.SMTPHostname}
}

func (a *App) engineAdmin() (string, error) {
	var secret map[string]string
	if err := platform.LoadSecret(filepath.Join(a.data, "engine.dpapi"), &secret); err != nil {
		return "", err
	}
	return "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:"+secret["password"])), nil
}

func (a *App) availableMailDomain(ctx context.Context, c Config, domain string) error {
	if !domainPattern.MatchString(domain) || domain == c.Domain {
		return errors.New("Choose a new mail subdomain")
	}
	// Check all names we would create, including CNAME/TXT collisions. Never
	// repurpose an existing webmail hostname, even one from a different preview.
	for _, name := range []string{domain, "smtp." + domain, "_dmarc." + domain} {
		var records []struct {
			ID string `json:"id"`
		}
		if err := cloudflareCall(ctx, c.DNSToken, "GET", "/zones/"+c.DNSZone+"/dns_records?name="+url.QueryEscape(name)+"&per_page=1", nil, &records); err != nil {
			return err
		}
		if len(records) > 0 {
			return fmt.Errorf("%s already has DNS records; choose another subdomain", name)
		}
	}
	return nil
}

func (a *App) planMailDomain(w http.ResponseWriter, r *http.Request) {
	if !a.setupAllowed(w, r) {
		return
	}
	a.mu.RLock()
	c := a.cfg
	busy := a.hostingBusy
	a.mu.RUnlock()
	if !c.AutomaticHosting || !c.Configured || busy {
		fail(w, errors.New("Wait for managed Mail setup to finish before choosing a subdomain"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()
	var zone struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if err := cloudflareCall(ctx, c.DNSToken, "GET", "/zones/"+c.DNSZone, nil, &zone); err != nil {
		fail(w, err)
		return
	}
	if zone.Status != "active" || !domainPattern.MatchString(zone.Name) {
		fail(w, errors.New("The connected domain is not active"))
		return
	}
	plan := mailDomainPlan{PreviousDomain: c.Domain, Unavailable: []string{}}
	for _, label := range []string{"mail", "inbox", "post", "mail2"} {
		domain := label + "." + zone.Name
		err := a.availableMailDomain(ctx, c, domain)
		if err == nil {
			plan.Domain = domain
			plan.SMTPHostname = "smtp." + domain
			writeJSON(w, plan)
			return
		}
		if !strings.Contains(err.Error(), "already has DNS records") && domain != c.Domain {
			fail(w, err)
			return
		}
		plan.Unavailable = append(plan.Unavailable, domain)
	}
	fail(w, errors.New("The suggested mail subdomains are already used. Keep existing DNS and choose a different dedicated mail domain with the Workspace owner"))
}

func (a *App) useMailDomain(w http.ResponseWriter, r *http.Request) {
	if !a.setupAllowed(w, r) {
		return
	}
	var in struct {
		Domain         string `json:"domain"`
		PreviousDomain string `json:"previousDomain"`
	}
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	a.mu.RLock()
	c := a.cfg
	busy := a.hostingBusy
	a.mu.RUnlock()
	if busy || !c.Configured || !c.AutomaticHosting {
		fail(w, errors.New("Wait for managed Mail setup to finish"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	journalPath := filepath.Join(a.data, "mail-domain-change.dpapi")
	var change mailDomainChange
	err := platform.LoadSecret(journalPath, &change)
	if err == nil {
		if change.Domain != in.Domain {
			fail(w, errors.New("Finish the pending mail-domain change before choosing another domain"))
			return
		}
	} else if os.IsNotExist(err) {
		if in.PreviousDomain != c.Domain {
			fail(w, errors.New("Mail settings changed. Review the subdomain choice again"))
			return
		}
		var zone struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		}
		if err = cloudflareCall(ctx, c.DNSToken, "GET", "/zones/"+c.DNSZone, nil, &zone); err != nil {
			fail(w, err)
			return
		}
		if zone.Status != "active" || !strings.HasSuffix(in.Domain, "."+zone.Name) || len(in.Domain) > 220 {
			fail(w, errors.New("Choose a mail subdomain inside your connected domain"))
			return
		}
		if err = a.availableMailDomain(ctx, c, in.Domain); err != nil {
			fail(w, err)
			return
		}
		var receipts hostingState
		if err = platform.LoadJSON(filepath.Join(a.data, "hosting-dns.json"), &receipts); err != nil && !os.IsNotExist(err) {
			fail(w, err)
			return
		}
		if len(receipts.Records) > 0 {
			fail(w, errors.New("This Mail domain already published DNS. Changing it requires a planned migration; its existing records are preserved"))
			return
		}
		auth, e := a.engineAdmin()
		if e != nil {
			fail(w, e)
			return
		}
		domains, e := a.jmap(ctx, auth, "x:Domain/get", map[string]any{})
		if e != nil {
			fail(w, e)
			return
		}
		items, _ := domains["list"].([]any)
		if len(items) != 1 {
			fail(w, errors.New("This installation has multiple mail domains. Keep them intact and use a planned migration for a separate mail domain"))
			return
		}
		for _, item := range items {
			d, ok := item.(map[string]any)
			if ok {
				aliases, _ := d["aliases"].(map[string]any)
				if d["name"] == in.Domain || aliases[in.Domain] != nil {
					fail(w, errors.New("The proposed subdomain is already used inside Mail"))
					return
				}
			}
			if !ok || d["name"] != c.Domain {
				continue
			}
			// This fallback is for an unpublished local domain, not a live mail move.
			for _, property := range []string{"dnsManagement", "dkimManagement", "certificateManagement"} {
				mode, _ := d[property].(map[string]any)
				if mode["@type"] != "Manual" {
					fail(w, errors.New("This domain already has managed signing, DNS or certificates. Use a planned migration"))
					return
				}
			}
			change = mailDomainChange{PreviousDomain: c.Domain, Domain: in.Domain, SMTPHostname: "smtp." + in.Domain, Zone: c.DNSZone, Issuer: c.Issuer, ClientID: c.ClientID}
			change.DomainID, _ = d["id"].(string)
			change.DirectoryID, _ = d["directoryId"].(string)
		}
		if change.DomainID == "" || change.DirectoryID == "" {
			fail(w, errors.New("The managed MyAccounts mail domain could not be found"))
			return
		}
		directories, e := a.jmap(ctx, auth, "x:Directory/get", map[string]any{"ids": []string{change.DirectoryID}})
		if e != nil {
			fail(w, e)
			return
		}
		list, _ := directories["list"].([]any)
		if len(list) != 1 {
			fail(w, errors.New("The managed MyAccounts directory is missing"))
			return
		}
		directory := list[0].(map[string]any)
		if directory["issuerUrl"] != c.Issuer || directory["requireAudience"] != c.ClientID || directory["claimUsername"] != "sub" || directory["usernameDomain"] != c.Domain {
			fail(w, errors.New("The mail directory needs owner review before changing domains"))
			return
		}
		system, e := a.jmap(ctx, auth, "x:SystemSettings/get", map[string]any{"ids": []string{"singleton"}})
		if e != nil {
			fail(w, e)
			return
		}
		list, _ = system["list"].([]any)
		if len(list) != 1 || list[0].(map[string]any)["defaultDomainId"] != change.DomainID {
			fail(w, errors.New("This is not the managed default mail domain; use a planned migration"))
			return
		}
		if err = platform.SaveSecret(journalPath, change); err != nil {
			fail(w, err)
			return
		}
	} else {
		fail(w, err)
		return
	}
	if _, err = a.resumeMailDomainChange(ctx); err != nil {
		a.mu.Lock()
		a.identityError = "Mail-domain change paused: " + err.Error()
		a.mu.Unlock()
		fail(w, err)
		return
	}
	a.beginHosting(context.Background())
	writeJSON(w, map[string]any{"ok": true, "domain": change.Domain, "message": "Mailboxes retained. Setting up the separate mail domain; the original domain’s DNS is preserved."})
}

// A protected journal lets startup finish the same in-place domain rename after
// interruption. Account/domain IDs and message storage are never recreated.
func (a *App) resumeMailDomainChange(ctx context.Context) (bool, error) {
	path := filepath.Join(a.data, "mail-domain-change.dpapi")
	var change mailDomainChange
	if err := platform.LoadSecret(path, &change); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	a.mu.RLock()
	c := a.cfg
	a.mu.RUnlock()
	if (c.Domain != change.PreviousDomain && c.Domain != change.Domain) || c.DNSZone != change.Zone || c.Issuer != change.Issuer || c.ClientID != change.ClientID {
		return false, errors.New("The pending mail-domain change does not match this installation")
	}
	auth, err := a.engineAdmin()
	if err != nil {
		return false, err
	}
	var domains map[string]any
	for attempt := 0; attempt < 60; attempt++ {
		domains, err = a.jmap(ctx, auth, "x:Domain/get", map[string]any{"ids": []string{change.DomainID}})
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	if err != nil {
		return false, err
	}
	list, _ := domains["list"].([]any)
	if len(list) != 1 {
		return false, errors.New("The original mail domain is missing; no mailbox data was recreated")
	}
	name, _ := list[0].(map[string]any)["name"].(string)
	if name != change.PreviousDomain && name != change.Domain {
		return false, errors.New("The native domain changed outside the pending operation")
	}
	directories, err := a.jmap(ctx, auth, "x:Directory/get", map[string]any{"ids": []string{change.DirectoryID}})
	if err != nil {
		return false, err
	}
	list, _ = directories["list"].([]any)
	if len(list) != 1 {
		return false, errors.New("The MyAccounts directory is missing")
	}
	d := list[0].(map[string]any)
	if d["issuerUrl"] != c.Issuer || d["requireAudience"] != c.ClientID || d["claimUsername"] != "sub" || (d["usernameDomain"] != change.PreviousDomain && d["usernameDomain"] != change.Domain) {
		return false, errors.New("The MyAccounts directory changed outside the pending operation")
	}
	if _, err = a.jmap(ctx, auth, "x:Domain/set", map[string]any{"update": map[string]any{change.DomainID: map[string]any{"name": change.Domain}}}); err != nil {
		return false, err
	}
	if _, err = a.jmap(ctx, auth, "x:Directory/set", map[string]any{"update": map[string]any{change.DirectoryID: map[string]any{"usernameDomain": change.Domain}}}); err != nil {
		return false, err
	}
	if _, err = a.jmap(ctx, auth, "x:SystemSettings/set", map[string]any{"update": map[string]any{"singleton": map[string]any{"defaultHostname": change.SMTPHostname}}}); err != nil {
		return false, err
	}
	if err = platform.SaveJSON(filepath.Join(a.data, "hosting-dns.json"), hostingState{Domain: change.Domain, Zone: change.Zone, Records: []hostedRecord{}}); err != nil {
		return false, err
	}
	c.Domain = change.Domain
	c.EngineHostname = change.SMTPHostname
	if err = platform.SaveSecret(filepath.Join(a.data, "mail.dpapi"), c); err != nil {
		return false, err
	}
	a.mu.Lock()
	a.cfg = c
	a.sessions = map[string]*Session{}
	a.identityError = ""
	a.mu.Unlock()
	a.stopEngine()
	if err = a.startEngine(); err != nil {
		return false, err
	}
	// Read back the same domain ID after the restart before clearing the journal.
	for attempt := 0; attempt < 60; attempt++ {
		domains, err = a.jmap(ctx, auth, "x:Domain/get", map[string]any{"ids": []string{change.DomainID}})
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	if err != nil {
		return false, err
	}
	list, _ = domains["list"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["name"] != change.Domain {
		return false, errors.New("The new mail domain was not saved")
	}
	if err = os.Remove(path); err != nil {
		return false, err
	}
	return true, nil
}
