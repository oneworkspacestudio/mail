// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Keep only bounded connection metadata in memory, never message content. A
// public port responding somewhere is not proof that it reaches this engine.
type smtpObservation struct {
	mu          sync.Mutex
	connections []smtpConnection
	receipts    map[string]time.Time
}
type smtpConnection struct {
	ip string
	at time.Time
}

var smtpStart = regexp.MustCompile(`^\d{4}-\d\d-\d\dT[0-9:.]+Z (?:INFO|DEBUG) SMTP connection started \(smtp\.connection-start\) `)
var smtpPeer = regexp.MustCompile(`(?:^|, )remoteIp = ([0-9.]+)(?:,|\s*$)`)
var smtpListener = regexp.MustCompile(`(?:^|, )listenerId = "workspace-internet-smtp"(?:,|\s*$)`)
var smtpPort = regexp.MustCompile(`(?:^|, )localPort = 25(?:,|\s*$)`)
var smtpQueued = regexp.MustCompile(`^\d{4}-\d\d-\d\dT[0-9:.]+Z INFO Queued message for delivery \(queue\.message-queued\) `)

// Accepted Internet SMTP for a local mail domain is independent evidence when
// the external checking service is unavailable. Retain only domain/time, never
// sender, recipient, message IDs or content. Explicit failed probes still win.
func (o *smtpObservation) observeReceipt(line string) {
	loc := smtpQueued.FindStringIndex(line)
	if loc == nil {
		return
	}
	fields := line[loc[1]:]
	peer := smtpPeer.FindStringSubmatch(fields)
	if len(peer) != 2 || !smtpListener.MatchString(fields) || !smtpPort.MatchString(fields) || !publicMailIP(net.ParseIP(peer[1])) {
		return
	}
	at, err := time.Parse(time.RFC3339Nano, strings.Fields(line)[0])
	if err != nil || at.After(time.Now().Add(time.Second)) || time.Since(at) > 2*time.Hour {
		return
	}
	start := strings.Index(fields, ", to = [")
	if start < 0 {
		return
	}
	var recipients []string
	if json.NewDecoder(strings.NewReader(fields[start+7:])).Decode(&recipients) != nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.receipts == nil {
		o.receipts = map[string]time.Time{}
	}
	for domain, seen := range o.receipts {
		if time.Since(seen) > 2*time.Hour {
			delete(o.receipts, domain)
		}
	}
	for _, recipient := range recipients {
		index := strings.LastIndex(recipient, "@")
		if index < 0 {
			continue
		}
		domain := strings.ToLower(recipient[index+1:])
		if !domainPattern.MatchString(domain) || len(o.receipts) >= 32 && o.receipts[domain].IsZero() {
			continue
		}
		if at.After(o.receipts[domain]) {
			o.receipts[domain] = at
		}
	}
}

func (o *smtpObservation) restoreReceipts(path string) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	start := info.Size() - 256*1024
	if start < 0 {
		start = 0
	}
	b := make([]byte, info.Size()-start)
	n, _ := f.ReadAt(b, start)
	lines := strings.Split(string(b[:n]), "\n")
	if start > 0 && len(lines) > 0 {
		lines = lines[1:]
	}
	for _, line := range lines {
		o.observeReceipt(line)
	}
}

func (a *App) incomingReceiptStatus(state, domain string, help inboundHelp) (string, string, inboundHelp, bool) {
	if state != "unverified" && state != "reachable" && state != "receiving" {
		return state, "", help, false
	}
	if help.Code != "unverified" && help.Code != "reachable" && help.Code != "receiving" && help.Code != "check_unavailable" && help.Code != "receiver_unverified" && help.Code != "upstream_router" && help.Code != "router_permission" {
		return state, "", help, false
	}
	a.smtpObservation.mu.Lock()
	at := a.smtpObservation.receipts[strings.ToLower(domain)]
	a.smtpObservation.mu.Unlock()
	if at.IsZero() || time.Since(at) > 2*time.Hour {
		return state, "", help, false
	}
	message := "This server accepted Internet email for @" + domain + " at " + at.UTC().Format("2 Jan 2006 15:04 UTC") + ". Automatic connection checks continue. This does not guarantee future delivery or spam placement."
	h := incomingHelp("receiving", nil)
	if !help.NextCheck.IsZero() {
		h.NextCheck = help.NextCheck
	}
	return "receiving", message, h, true
}

func (o *smtpObservation) observe(line string) {
	o.observeReceipt(line)
	loc := smtpStart.FindStringIndex(line)
	if loc == nil {
		return
	}
	fields := line[loc[1]:]
	peer := smtpPeer.FindStringSubmatch(fields)
	if len(peer) != 2 || !smtpListener.MatchString(fields) || !smtpPort.MatchString(fields) || !publicMailIP(net.ParseIP(peer[1])) {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.connections) >= 256 {
		copy(o.connections, o.connections[1:])
		o.connections = o.connections[:255]
	}
	o.connections = append(o.connections, smtpConnection{peer[1], time.Now()})
}

// Match the same successful node, target address and current probe window. Old
// mail, unrelated Internet scanners, LAN tests and another PC's open port cannot
// satisfy this check. The node metadata comes from the existing HTTPS probe API.
func (o *smtpObservation) confirms(since time.Time, nodes map[string][]string, results map[string]json.RawMessage, targets []net.IP) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	for node, meta := range nodes {
		if len(meta) < 4 || !publicMailIP(net.ParseIP(meta[3])) {
			continue
		}
		var entries []struct {
			Time    *float64 `json:"time"`
			Error   string   `json:"error"`
			Address string   `json:"address"`
		}
		if json.Unmarshal(results[node], &entries) != nil {
			continue
		}
		for _, result := range entries {
			if result.Time == nil || result.Error != "" {
				continue
			}
			matchedTarget := false
			for _, target := range targets {
				if target.Equal(net.ParseIP(result.Address)) {
					matchedTarget = true
				}
			}
			if !matchedTarget {
				continue
			}
			for _, connection := range o.connections {
				if connection.ip == meta[3] && !connection.at.Before(since) && time.Since(connection.at) < 90*time.Second {
					return true
				}
			}
		}
	}
	return false
}

// Enable just the connection-start metadata event in Workspace's existing
// stdout tracer. Never enable raw SMTP/debug payload logging. Registry reload
// applies this to upgrades and fresh installations without restarting Mail.
func (a *App) ensureSMTPObservation(ctx context.Context) error {
	auth, err := a.engineAdmin()
	if err != nil {
		return err
	}
	got, err := a.jmap(ctx, auth, "x:EventTracingLevel/get", map[string]any{})
	if err != nil {
		return err
	}
	list, _ := got["list"].([]any)
	id := ""
	ready := false
	for _, item := range list {
		v, ok := item.(map[string]any)
		if !ok || v["event"] != "smtp.connection-start" {
			continue
		}
		if id != "" {
			return errors.New("duplicate SMTP connection metadata policy")
		}
		id, _ = v["id"].(string)
		ready = v["level"] == "info"
	}
	if ready {
		return nil
	}
	args := map[string]any{"create": map[string]any{"workspace-probe": map[string]any{"event": "smtp.connection-start", "level": "info"}}}
	if id != "" {
		args = map[string]any{"update": map[string]any{id: map[string]any{"level": "info"}}}
	}
	if _, err = a.jmap(ctx, auth, "x:EventTracingLevel/set", args); err != nil {
		return err
	}
	_, err = a.jmap(ctx, auth, "x:Action/set", map[string]any{"create": map[string]any{"workspace-probe": map[string]any{"@type": "ReloadSettings"}}})
	return err
}
