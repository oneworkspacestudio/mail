// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"errors"
	"net"
	"net/http"
	"time"
)

type inboundRouteIssue struct{ Code, Message, Router, Target string }

func (e *inboundRouteIssue) Error() string { return e.Message }

type inboundHelp struct {
	Code      string    `json:"code"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	RouterURL string    `json:"routerURL,omitempty"`
	Target    string    `json:"target,omitempty"`
	NextCheck time.Time `json:"nextCheck,omitempty"`
}

func incomingHelp(state string, err error) inboundHelp {
	if err != nil && state == "reachable" {
		state = "unverified"
	}
	help := inboundHelp{Code: state, Title: "Checking incoming mail", Summary: "Mail is checking the connection and repairing its own router rules automatically."}
	switch state {
	case "reachable":
		help.Title = "Incoming connection is available"
		help.Summary = "The public connection is reachable. This does not yet confirm message delivery."
	case "unverified":
		help.Title = "Incoming connection is not verified yet"
		help.Summary = "The external connection check has not returned a conclusive result. Mail will check again automatically."
	case "attention":
		help.Title = "Incoming connection needs attention"
		help.Summary = "Mail could not establish a public incoming connection. Automatic repair will retry."
	}
	var issue *inboundRouteIssue
	if errors.As(err, &issue) {
		help.Code = issue.Code
		if ip := net.ParseIP(issue.Router); ip != nil && ip.IsPrivate() && !ip.IsLoopback() {
			help.RouterURL = "http://" + ip.String() + "/"
		}
		if ip := net.ParseIP(issue.Target); ip != nil && ip.IsPrivate() && !ip.IsLoopback() {
			help.Target = ip.String()
		}
		switch issue.Code {
		case "router_permission":
			help.Title = "Allow automatic setup on your router"
			help.Summary = "Your router is not accepting automatic setup requests. Open its settings and enable UPnP, then choose Repair connection. Mail cannot change a router that requires its owner’s permission."
		case "other_mail_server":
			help.Title = "Another PC already receives your email"
			help.Summary = "Keep the current receiver and connect both installations through it, or deliberately move incoming mail to this PC. A new email subdomain cannot split a single router connection between two PCs. Existing forwarding has been preserved."
		case "upstream_router":
			help.Title = "One more router needs permission"
			help.Summary = "Mail configured the local router, but the upstream router still needs a confirmed route to it. Existing routes are preserved."
		case "dns_pending":
			help.Title = "Waiting for the email address to connect"
			help.Summary = "Mail is waiting for public DNS to match this connection. It will retry automatically."
		}
	}
	help.NextCheck = time.Now().UTC().Add(5 * time.Minute)
	return help
}

// Recheck only the network; do not rerun DNS/native account setup for a router
// problem. The existing worker serializes and coalesces repair requests.
func (a *App) retryIncoming(w http.ResponseWriter, r *http.Request) {
	if !a.setupAllowed(w, r) {
		return
	}
	a.mu.RLock()
	managed := a.cfg.AutomaticHosting && a.cfg.MainMailURL == ""
	a.mu.RUnlock()
	if !managed {
		fail(w, errors.New("Incoming setup is managed by the Workspace owner"))
		return
	}
	a.inboundProgress("checking", "Checking and repairing the incoming mail connection…")
	a.mu.Lock()
	a.inboundHelp = incomingHelp("checking", nil)
	a.inboundHelp.NextCheck = time.Time{}
	a.mu.Unlock()
	select {
	case a.inboundRetry <- struct{}{}:
	default:
	}
	w.WriteHeader(http.StatusAccepted)
}
