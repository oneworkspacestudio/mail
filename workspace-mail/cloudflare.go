// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type DNSInput struct {
	Token     string `json:"token"`
	ZoneID    string `json:"zoneId"`
	IP        string `json:"ip"`
	WebTarget string `json:"webTarget"`
	Confirm   bool   `json:"confirm"`
}
type DNSRecord struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	Proxied  bool   `json:"proxied"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority,omitempty"`
}

var zonePattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func cloudflareCall(ctx context.Context, token, method, path string, body, out any) error {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	req, e := http.NewRequestWithContext(ctx, method, "https://api.cloudflare.com/client/v4"+path, bytes.NewReader(b))
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, e := (&http.Client{Timeout: 25e9, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
	if e != nil {
		return errors.New("Cloudflare is unreachable")
	}
	defer resp.Body.Close()
	var result struct {
		Success bool            `json:"success"`
		Result  json.RawMessage `json:"result"`
	}
	if e = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); e != nil || !result.Success || resp.StatusCode >= 300 {
		return fmt.Errorf("Cloudflare rejected %s (HTTP %d); check zone ownership and token DNS permissions", method, resp.StatusCode)
	}
	if out != nil {
		return json.Unmarshal(result.Result, out)
	}
	return nil
}
func (a *App) dnsRecords(in DNSInput) ([]DNSRecord, error) {
	a.mu.RLock()
	c := a.cfg
	a.mu.RUnlock()
	if !c.Configured {
		return nil, errors.New("Complete Mail setup first")
	}
	if !zonePattern.MatchString(in.ZoneID) || in.Token == "" {
		return nil, errors.New("Enter a Cloudflare Zone ID and API token")
	}
	var records []DNSRecord
	if in.WebTarget != "" {
		if !domainPattern.MatchString(in.WebTarget) {
			return nil, errors.New("Web target must be a tunnel CNAME hostname")
		}
		records = append(records, DNSRecord{Type: "CNAME", Name: c.Hostname, Content: in.WebTarget, Proxied: true, TTL: 1})
	}
	if in.IP != "" {
		ip := net.ParseIP(in.IP)
		if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() {
			return nil, errors.New("Enter the public IP of your SMTP server")
		}
		typ := "A"
		if ip.To4() == nil {
			typ = "AAAA"
		}
		records = append(records, DNSRecord{Type: typ, Name: "smtp." + c.Domain, Content: ip.String(), TTL: 1}, DNSRecord{Type: "MX", Name: c.Domain, Content: "smtp." + c.Domain, Priority: 10, TTL: 1}, DNSRecord{Type: "TXT", Name: c.Domain, Content: "v=spf1 mx -all", TTL: 1}, DNSRecord{Type: "TXT", Name: "_dmarc." + c.Domain, Content: "v=DMARC1; p=none", TTL: 1})
	}
	if len(records) == 0 {
		return nil, errors.New("Provide a web CNAME target or a public SMTP server IP")
	}
	return records, nil
}
func (a *App) dnsPlan(w http.ResponseWriter, r *http.Request) {
	if !a.setupAllowed(w, r) {
		return
	}
	var in DNSInput
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	records, e := a.dnsRecords(in)
	if e != nil {
		fail(w, e)
		return
	}
	writeJSON(w, map[string]any{"records": records, "note": "Review before applying. Existing records will never be replaced. SMTP records use DNS only. DKIM keys must also be published from Stalwart administration; SMTP TLS and reverse DNS are required for Internet delivery."})
}
func (a *App) dnsApply(w http.ResponseWriter, r *http.Request) {
	if !a.setupAllowed(w, r) {
		return
	}
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	var in DNSInput
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	if !in.Confirm {
		fail(w, errors.New("Review and confirm the DNS plan first"))
		return
	}
	records, e := a.dnsRecords(in)
	if e != nil {
		fail(w, e)
		return
	}
	var zone struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if e = cloudflareCall(r.Context(), in.Token, "GET", "/zones/"+in.ZoneID, nil, &zone); e != nil {
		fail(w, e)
		return
	}
	if zone.Status != "active" {
		fail(w, errors.New("Cloudflare zone is not active"))
		return
	}
	a.mu.RLock()
	domain := a.cfg.Domain
	a.mu.RUnlock()
	if domain != zone.Name && !strings.HasSuffix(domain, "."+zone.Name) {
		fail(w, errors.New("Mail domain does not belong to this zone"))
		return
	}
	// Preflight the whole plan so a known collision cannot produce partial changes.
	for _, record := range records {
		if record.Name != zone.Name && !strings.HasSuffix(record.Name, "."+zone.Name) {
			fail(w, errors.New("Web hostname is outside this zone"))
			return
		}
		var existing []struct {
			ID string `json:"id"`
		}
		if e = cloudflareCall(r.Context(), in.Token, "GET", "/zones/"+in.ZoneID+"/dns_records?name="+url.QueryEscape(record.Name), nil, &existing); e != nil {
			fail(w, e)
			return
		}
		if len(existing) > 0 {
			fail(w, fmt.Errorf("DNS collision at %s; no records have been changed", record.Name))
			return
		}
	}
	var created []string
	for _, record := range records {
		var item struct {
			ID string `json:"id"`
		}
		if e = cloudflareCall(r.Context(), in.Token, "POST", "/zones/"+in.ZoneID+"/dns_records", record, &item); e != nil {
			fail(w, fmt.Errorf("%w; %d records created before this failure. Inspect Cloudflare before retrying", e, len(created)))
			return
		}
		created = append(created, item.ID)
	}
	writeJSON(w, map[string]any{"ok": true, "created": len(created), "records": records})
}
