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
	if a.sharedRoot != "" {
		return "", errors.New("Recovery credentials belong to Workspace Services; Mail has no shared administrator credential")
	}
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

func (a *App) chooseMailDomain(ctx context.Context, c Config) (mailDomainPlan, error) {
	var zone struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	plan := mailDomainPlan{PreviousDomain: c.Domain, Unavailable: []string{}}
	if err := cloudflareCall(ctx, c.DNSToken, "GET", "/zones/"+c.DNSZone, nil, &zone); err != nil {
		return plan, err
	}
	if zone.Status != "active" || !domainPattern.MatchString(zone.Name) {
		return plan, errors.New("The connected domain is not active")
	}
	labels := []string{"mail", "inbox", "post", "mailbox", "mail2"}
	for n := 3; n <= 20; n++ {
		labels = append(labels, fmt.Sprintf("mail%d", n))
	}
	for _, label := range labels {
		domain := label + "." + zone.Name
		err := a.availableMailDomain(ctx, c, domain)
		if err == nil {
			plan.Domain = domain
			plan.SMTPHostname = "smtp." + domain
			return plan, nil
		}
		if !strings.Contains(err.Error(), "already has DNS records") && domain != c.Domain {
			return plan, err
		}
		plan.Unavailable = append(plan.Unavailable, domain)
	}
	return plan, errors.New("No unused mail address was found. Open advanced settings to choose another mail domain; existing DNS is unchanged")
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
		fail(w, errors.New("Wait for managed Mail setup to finish"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	plan, err := a.chooseMailDomain(ctx, c)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, plan)
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
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	if err := a.applyMailDomain(ctx, in.Domain, in.PreviousDomain, false); err != nil {
		fail(w, err)
		return
	}
	a.beginHosting(context.Background())
	writeJSON(w, map[string]any{"ok": true, "domain": in.Domain, "message": "Mailboxes retained. Finishing automatic Mail setup; existing DNS is preserved."})
}

// The same journalled operation serves first-run automation and owner recovery.
// Published domains and multi-domain installations are never renamed implicitly.
func (a *App) applyMailDomain(ctx context.Context, domain, previousDomain string, automatic bool) error {
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	a.mu.RLock()
	c := a.cfg
	busy := a.hostingBusy
	a.mu.RUnlock()
	if (busy && !automatic) || !c.Configured || !c.AutomaticHosting {
		return errors.New("Wait for managed Mail setup to finish")
	}
	journalPath := filepath.Join(a.data, "mail-domain-change.dpapi")
	var change mailDomainChange
	err := platform.LoadSecret(journalPath, &change)
	if err == nil {
		if change.Domain != domain {
			return errors.New("Finish the pending mail-domain change before choosing another domain")
		}
	} else if os.IsNotExist(err) {
		if previousDomain != c.Domain {
			return errors.New("Mail settings changed. Review the subdomain choice again")
		}
		var zone struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		}
		if err = cloudflareCall(ctx, c.DNSToken, "GET", "/zones/"+c.DNSZone, nil, &zone); err != nil {
			return err
		}
		if zone.Status != "active" || !strings.HasSuffix(domain, "."+zone.Name) || len(domain) > 220 {
			return errors.New("Choose a mail subdomain inside your connected domain")
		}
		if err = a.availableMailDomain(ctx, c, domain); err != nil {
			return err
		}
		var receipts hostingState
		if err = platform.LoadJSON(filepath.Join(a.data, "hosting-dns.json"), &receipts); err != nil && !os.IsNotExist(err) {
			return err
		}
		if len(receipts.Records) > 0 {
			return errors.New("This Mail domain already published DNS. Changing it requires a planned migration; its existing records are preserved")
		}
		auth, e := a.engineAdmin()
		if e != nil {
			return e
		}
		domains, e := a.jmap(ctx, auth, "x:Domain/get", map[string]any{})
		if e != nil {
			return e
		}
		items, _ := domains["list"].([]any)
		if len(items) != 1 {
			return errors.New("This installation has multiple mail domains. Keep them intact and use a planned migration for a separate mail domain")
		}
		for _, item := range items {
			d, ok := item.(map[string]any)
			if ok {
				aliases, _ := d["aliases"].(map[string]any)
				if d["name"] == domain || aliases[domain] != nil {
					return errors.New("The proposed subdomain is already used inside Mail")
				}
			}
			if !ok || d["name"] != c.Domain {
				continue
			}
			// This fallback is for an unpublished local domain, not a live mail move.
			for _, property := range []string{"dnsManagement", "dkimManagement", "certificateManagement"} {
				mode, _ := d[property].(map[string]any)
				if mode["@type"] != "Manual" {
					return errors.New("This domain already has managed signing, DNS or certificates. Use a planned migration")
				}
			}
			change = mailDomainChange{PreviousDomain: c.Domain, Domain: domain, SMTPHostname: "smtp." + domain, Zone: c.DNSZone, Issuer: c.Issuer, ClientID: c.ClientID}
			change.DomainID, _ = d["id"].(string)
			change.DirectoryID, _ = d["directoryId"].(string)
		}
		if change.DomainID == "" || change.DirectoryID == "" {
			return errors.New("The managed MyAccounts mail domain could not be found")
		}
		directories, e := a.jmap(ctx, auth, "x:Directory/get", map[string]any{"ids": []string{change.DirectoryID}})
		if e != nil {
			return e
		}
		list, _ := directories["list"].([]any)
		if len(list) != 1 {
			return errors.New("The managed MyAccounts directory is missing")
		}
		directory := list[0].(map[string]any)
		if directory["issuerUrl"] != c.Issuer || directory["requireAudience"] != c.ClientID || directory["claimUsername"] != "sub" || directory["usernameDomain"] != c.Domain {
			return errors.New("The mail directory needs owner review before changing domains")
		}
		system, e := a.jmap(ctx, auth, "x:SystemSettings/get", map[string]any{"ids": []string{"singleton"}})
		if e != nil {
			return e
		}
		list, _ = system["list"].([]any)
		if len(list) != 1 || list[0].(map[string]any)["defaultDomainId"] != change.DomainID {
			return errors.New("This is not the managed default mail domain; use a planned migration")
		}
		if err = platform.SaveSecret(journalPath, change); err != nil {
			return err
		}
	} else {
		return err
	}
	if _, err = a.resumeMailDomainChange(ctx); err != nil {
		a.mu.Lock()
		a.identityError = "Mail-domain change paused: " + err.Error()
		a.mu.Unlock()
		return err
	}
	return nil
}

// A protected journal lets startup finish the same in-place domain rename after
// interruption. Account/domain IDs and message storage are never recreated.
func (a *App) resumeMailDomainChange(ctx context.Context) (bool, error) {
	a.provisionMu.Lock()
	defer a.provisionMu.Unlock()
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
	// Validate and retain human-readable mailbox assignments before changing
	// anything native. Domain IDs and account IDs stay the same. The journal
	// permits either side after an interrupted write, without losing assignments.
	bindingsPath := filepath.Join(a.data, "mailboxes.dpapi")
	var bindings MailboxBindings
	bindingsErr := platform.LoadSecret(bindingsPath, &bindings)
	if bindingsErr != nil && !os.IsNotExist(bindingsErr) {
		return false, errors.New("Mailbox assignments could not be read; reconnect Mail before changing its domain")
	}
	if bindingsErr == nil {
		if bindings.Issuer != c.Issuer || bindings.ClientID != c.ClientID || (bindings.Domain != change.PreviousDomain && bindings.Domain != change.Domain) {
			return false, errors.New("Mailbox assignments do not match the pending mail-domain change")
		}
		for subject, binding := range bindings.Users {
			suffix := "@" + bindings.Domain
			if !strings.HasSuffix(binding.Address, suffix) || strings.Count(binding.Address, "@") != 1 {
				return false, errors.New("A mailbox address needs review before changing its domain")
			}
			binding.Address = strings.TrimSuffix(binding.Address, suffix) + "@" + change.Domain
			bindings.Users[subject] = binding
		}
		bindings.Domain = change.Domain
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
	if bindingsErr == nil {
		if err = platform.SaveSecret(bindingsPath, bindings); err != nil {
			return false, err
		}
	}
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
