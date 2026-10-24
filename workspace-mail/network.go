// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const smtpMappingDescription = "One Workspace Mail inbound SMTP"

type mailGateway struct{ Service, Control string }
type igdService struct {
	Type    string `xml:"serviceType"`
	Control string `xml:"controlURL"`
}
type igdDevice struct {
	Services []igdService `xml:"serviceList>service"`
	Devices  []igdDevice  `xml:"deviceList>device"`
}
type igdDescription struct {
	Device  igdDevice `xml:"device"`
	URLBase string    `xml:"URLBase"`
}

func (a *App) inboundProgress(state, message string) {
	a.mu.Lock()
	a.inboundState = state
	a.inboundMessage = message
	if state == "checking" {
		a.inboundHelp = incomingHelp(state, nil)
		a.inboundHelp.NextCheck = time.Time{}
	}
	a.mu.Unlock()
}

// DNS and an HTTPS tunnel do not open SMTP through NAT. Maintain a leased,
// conflict-checked mapping only for the managed engine's public TCP25 listener.
func (a *App) maintainInboundMail(ctx context.Context) {
	for {
		a.networkMu.Lock()
		a.mu.RLock()
		c := a.cfg
		previousState := a.inboundState
		previousHelp := a.inboundHelp
		chooseMain := a.hostingIssue != nil && a.hostingIssue.Code == "main_mail_available"
		a.mu.RUnlock()
		if effective, _, _, ok := a.incomingReceiptStatus(previousState, c.Domain, previousHelp); ok {
			previousState = effective
		}
		delay := 5 * time.Minute
		if c.MainMailURL != "" {
			a.inboundProgress("shared", "Incoming mail is managed by the main Mail service.")
		} else if chooseMain {
			a.inboundProgress("choose_service", "Choose whether to use the existing main Mail service.")
		} else if !c.Configured {
			a.inboundProgress("checking", "Waiting for the mail server to finish setup.")
			delay = 5 * time.Second
		} else {
			// Routine monitoring must not make a verified working service look
			// broken while its next check is in progress.
			if previousState != "verified" && previousState != "receiving" {
				a.inboundProgress("checking", "Checking router forwarding and public TCP25 reachability…")
			}
			attempt, cancel := context.WithTimeout(ctx, 90*time.Second)
			a.mu.Lock()
			a.networkCancel = cancel
			a.mu.Unlock()
			ownerHash := sha256.Sum256([]byte(a.data + "|" + c.ClientID))
			// Stay below routers' common 31-character description limit.
			description := "Workspace Mail " + hex.EncodeToString(ownerHash[:6])
			routeMessage, routes, routeErr := configureInboundRoute(attempt, c.EngineHostname, description)
			if !a.probeLoggingReady {
				a.probeLoggingReady = a.ensureSMTPObservation(attempt) == nil
			}
			state, probeMessage := a.checkIncomingSMTP(attempt, c.EngineHostname)
			if state == "attention" && routeErr == nil && len(routes) > 0 {
				a.inboundProgress("checking", "The public connection failed. Refreshing this installation's verified router rules and checking again…")
				if err := refreshOwnedMailRoutes(attempt, routes); err == nil {
					state, probeMessage = a.checkIncomingSMTP(attempt, c.EngineHostname)
				} else {
					routeErr = fmt.Errorf("Automatic router recovery could not finish: %w. Review incoming Mail settings", err)
				}
			}
			cancel()
			help := incomingHelp(state, routeErr)
			if effective, message, receiptHelp, ok := a.incomingReceiptStatus(state, c.Domain, help); ok {
				state, probeMessage, help, routeErr = effective, message, receiptHelp, nil
			}
			a.mu.Lock()
			a.networkCancel = nil
			if state == "verified" {
				routeErr = nil
			}
			if state == "verified" {
				help = incomingHelp(state, nil)
			}
			a.inboundHelp = help
			a.mu.Unlock()
			if state == "verified" || state == "receiving" {
				a.inboundProgress(state, probeMessage)
			} else if state == "reachable" && routeErr == nil {
				a.inboundProgress(state, probeMessage+" "+routeMessage)
			} else if routeErr != nil {
				if state == "reachable" {
					state = "unverified"
					probeMessage = "Public SMTP responds. This server has not yet matched the external check; incoming email can confirm the working route automatically."
				}
				if help.Code == "check_unavailable" || help.Code == "receiver_unverified" {
					a.inboundProgress(state, probeMessage+" Automatic router inspection is unavailable; existing settings are preserved.")
				} else {
					a.inboundProgress(state, routeErr.Error()+" "+probeMessage)
				}
			} else {
				a.inboundProgress(state, routeMessage+" "+probeMessage)
			}
		}
		a.networkMu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		case <-a.inboundRetry:
		}
	}
}

// The public probe receives only the public SMTP hostname, never messages or
// credentials. A TCP connection proves reachability, not successful delivery.
func (a *App) checkIncomingSMTP(ctx context.Context, hostname string) (string, string) {
	addresses, err := net.DefaultResolver.LookupIP(ctx, "ip4", hostname)
	if err != nil || len(addresses) == 0 {
		return "unverified", "The SMTP DNS address could not be checked. Setup will retry automatically."
	}
	for _, ip := range addresses {
		if !publicMailIP(ip) {
			return "attention", "The SMTP hostname needs a public IPv4 address. Private or shared ISP addresses cannot receive Internet email directly."
		}
	}
	client := &http.Client{Timeout: 6 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	get := func(endpoint string, out any) error {
		req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("probe HTTP %d", resp.StatusCode)
		}
		return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
	}
	since := time.Now()
	var started struct {
		ID    string              `json:"request_id"`
		Nodes map[string][]string `json:"nodes"`
	}
	if err = get("https://check-host.net/check-tcp?host="+url.QueryEscape(net.JoinHostPort(hostname, "25"))+"&max_nodes=3", &started); err != nil || !validProbeID(started.ID) {
		return "unverified", "External connection check is unavailable. Incoming mail is not yet verified; setup retries automatically. The check shares only your public SMTP hostname with Check-Host."
	}
	for n := 0; n < 4; n++ {
		select {
		case <-ctx.Done():
			return "unverified", "The external connection check timed out. Setup will retry automatically."
		case <-time.After(2 * time.Second):
		}
		var results map[string]json.RawMessage
		if err = get("https://check-host.net/check-result/"+started.ID, &results); err != nil {
			continue
		}
		state := incomingProbeState(results)
		if state == "reachable" {
			// The external TCP check can finish before the engine flushes its
			// connection event. Wait briefly for the matching local observation.
			for wait := 0; wait < 20; wait++ {
				if a.smtpObservation.confirms(since, started.Nodes, results, addresses) {
					return "verified", "An external check reached this Mail server on TCP25. Your incoming connection is working; no router change is needed. This checks the connection, not message delivery or spam placement."
				}
				select {
				case <-ctx.Done():
					return "unverified", "The external connection check timed out. Mail will check again automatically."
				case <-time.After(100 * time.Millisecond):
				}
			}
			return state, "Public TCP25 is reachable from an external network. This confirms the incoming connection, not message delivery or spam placement."
		}
		if state == "attention" {
			return state, "External networks could not connect to TCP25. In Windows Security allow the Mail server on this network, forward router TCP25 to this PC, and check your ISP permits incoming SMTP. If the ISP blocks it, use a public mail server or inbound gateway. Setup retries automatically."
		}
	}
	return "unverified", "No conclusive external result yet. Incoming mail is not verified; setup retries automatically. The check uses Check-Host with only your public SMTP hostname."
}

func publicMailIP(ip net.IP) bool {
	v4 := ip.To4()
	return v4 != nil && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !(v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127)
}

func validProbeID(id string) bool {
	if len(id) < 1 || len(id) > 80 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func incomingProbeState(results map[string]json.RawMessage) string {
	finished, failed := 0, 0
	for _, raw := range results {
		var entries []struct {
			Time    *float64 `json:"time"`
			Error   string   `json:"error"`
			Address string   `json:"address"`
		}
		if json.Unmarshal(raw, &entries) != nil || len(entries) == 0 {
			continue
		}
		finished++
		for _, entry := range entries {
			if entry.Time != nil && entry.Error == "" && publicMailIP(net.ParseIP(entry.Address)) {
				return "reachable"
			}
		}
		if entries[0].Error != "" {
			failed++
		}
	}
	if len(results) > 0 && finished == len(results) && failed == finished {
		return "attention"
	}
	return "unverified"
}

type smtpRoute struct {
	Gateway             mailGateway
	Target, Description string
}

func configureInboundRoute(ctx context.Context, hostname, description string) (string, []smtpRoute, error) {
	connection, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(1, 1, 1, 1), Port: 53})
	if err != nil {
		return "", nil, errors.New("Incoming mail needs a network connection. Setup will retry automatically.")
	}
	local := connection.LocalAddr().(*net.UDPAddr).IP
	connection.Close()
	if !local.IsPrivate() {
		return "This PC has a public network address. Incoming email still requires TCP25 through its firewall.", nil, nil
	}
	gateway, err := discoverMailGateway(ctx, local)
	if err != nil {
		return "", nil, &inboundRouteIssue{Code: "router_permission", Message: "The router did not accept automatic incoming-mail setup.", Router: mailDefaultGateway(local).String(), Target: local.String()}
	}
	addresses, err := net.DefaultResolver.LookupIP(ctx, "ip4", hostname)
	if err != nil {
		return "", nil, &inboundRouteIssue{Code: "dns_pending", Message: "Waiting for the email address to connect to this server."}
	}
	fields, err := gateway.action(ctx, "GetExternalIPAddress", nil)
	if err != nil {
		return "", nil, errors.New("The router could not report its Internet address. Review incoming Mail network settings.")
	}
	external := net.ParseIP(fields["NewExternalIPAddress"])
	if publicMailIP(external) {
		match := false
		for _, ip := range addresses {
			if ip.Equal(external) {
				match = true
			}
		}
		if !match {
			return "", nil, errors.New("Router public IP and SMTP DNS do not match. Review hosting DNS before receiving mail.")
		}
	}
	query := smtpMappingQuery()
	previous, _ := gateway.action(ctx, "GetSpecificPortMappingEntry", query)
	migrating := previous["NewInternalClient"] == local.String() && previous["NewInternalPort"] == "25" && previous["NewEnabled"] == "1" && previous["NewPortMappingDescription"] == smtpMappingDescription
	message, err := gateway.ensureSMTP(ctx, local.String(), description)
	if err != nil {
		return "", nil, err
	}
	var routes []smtpRoute
	current, _ := gateway.action(ctx, "GetSpecificPortMappingEntry", query)
	if current["NewPortMappingDescription"] == description {
		routes = append(routes, smtpRoute{gateway, local.String(), description})
	}
	for _, hop := range privateMailHops(ctx) {
		if sameMailSubnet(local, hop) {
			continue
		}
		upstream, e := discoverMailGateway(ctx, local, hop)
		if e != nil {
			return message, routes, &inboundRouteIssue{Code: "upstream_router", Message: "The upstream router did not accept automatic Mail setup. Its owner needs to allow the incoming connection; existing forwarding has been preserved.", Router: hop.String()}
		}
		var downstream net.IP
		if len(routes) > 0 && external.IsPrivate() {
			downstream = external
		}
		route, e := renewUpstreamSMTP(ctx, upstream, hop, description, migrating && len(routes) > 0, downstream)
		if e != nil {
			return message, routes, e
		}
		routes = append(routes, route)
	}
	if !publicMailIP(external) && len(routes) < 2 {
		return message, routes, errors.New("Shared or upstream NAT prevents a direct incoming route. A public address or upstream forwarding/inbound gateway is needed.")
	}
	if len(routes) > 1 {
		message = "TCP25 forwarding is configured across the detected router hops and renewed automatically. Public reachability is checked separately."
	}
	return message, routes, nil
}

func smtpMappingQuery() map[string]string {
	return map[string]string{"NewRemoteHost": "", "NewExternalPort": "25", "NewProtocol": "TCP"}
}

func renewUpstreamSMTP(ctx context.Context, gateway mailGateway, hop net.IP, description string, allowLegacy bool, downstream ...net.IP) (smtpRoute, error) {
	f, err := gateway.action(ctx, "GetSpecificPortMappingEntry", smtpMappingQuery())
	// The inner gateway's private WAN address proves the next target. Do not
	// guess from traceroute or replace another receiver's outer router rule.
	if len(downstream) > 0 && downstream[0] != nil {
		ip := downstream[0]
		verified := ip.IsPrivate() && !ip.Equal(hop) && ip.Mask(net.CIDRMask(24, 32)).Equal(hop.Mask(net.CIDRMask(24, 32)))
		var fault *gatewayError
		if verified && errors.As(err, &fault) && fault.Code == "714" {
			if _, e := gateway.ensureSMTP(ctx, ip.String(), description); e != nil {
				return smtpRoute{}, e
			}
			return smtpRoute{gateway, ip.String(), description}, nil
		}
		if verified && err == nil && f["NewInternalClient"] == ip.String() && f["NewInternalPort"] == "25" && f["NewEnabled"] == "1" && f["NewPortMappingDescription"] != description {
			// Compatible existing forwarding is used unchanged. Empty description
			// explicitly excludes it from this installation's destructive refresh.
			return smtpRoute{gateway, ip.String(), ""}, nil
		}
	}
	target := net.ParseIP(f["NewInternalClient"])
	sameSubnet := target != nil && target.To4() != nil && target.IsPrivate() && target.Mask(net.CIDRMask(24, 32)).Equal(hop.Mask(net.CIDRMask(24, 32))) && !target.Equal(hop)
	owned := f["NewPortMappingDescription"] == description || allowLegacy && f["NewPortMappingDescription"] == smtpMappingDescription
	if err != nil || !sameSubnet || !owned || f["NewInternalPort"] != "25" || f["NewEnabled"] != "1" {
		return smtpRoute{}, &inboundRouteIssue{Code: "upstream_router", Message: "An upstream router needs permission to finish incoming-mail setup.", Router: hop.String()}
	}
	_, err = gateway.ensureSMTP(ctx, target.String(), description)
	if err != nil {
		return smtpRoute{}, err
	}
	return smtpRoute{gateway, target.String(), description}, nil
}

func refreshOwnedMailRoutes(ctx context.Context, routes []smtpRoute) error {
	// Refresh outer rules first; only exact verified ownership and targets qualify.
	for i := len(routes) - 1; i >= 0; i-- {
		route := routes[i]
		if route.Description == "" {
			continue
		}
		f, err := route.Gateway.action(ctx, "GetSpecificPortMappingEntry", smtpMappingQuery())
		if err != nil || f["NewInternalClient"] != route.Target || f["NewInternalPort"] != "25" || f["NewEnabled"] != "1" || f["NewPortMappingDescription"] != route.Description {
			return errors.New("Router rule changed; automatic recovery preserved it")
		}
		if _, err = route.Gateway.action(ctx, "DeletePortMapping", smtpMappingQuery()); err != nil {
			return err
		}
		if _, err = route.Gateway.ensureSMTP(ctx, route.Target, route.Description); err != nil {
			return err
		}
	}
	return nil
}

func discoverMailGateway(ctx context.Context, local net.IP, upstream ...net.IP) (mailGateway, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: local})
	if err != nil {
		return mailGateway{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	targets := []net.IP{net.IPv4(239, 255, 255, 250)}
	if len(upstream) > 0 {
		targets = []net.IP{upstream[0]}
	} else if gateway := mailDefaultGateway(local); gateway != nil {
		targets = append(targets, gateway)
	}
	searches := []string{"urn:schemas-upnp-org:device:InternetGatewayDevice:1", "urn:schemas-upnp-org:service:WANIPConnection:1", "ssdp:all"}
	for _, destination := range targets {
		for _, search := range searches {
			request := "M-SEARCH * HTTP/1.1\r\nHOST: " + net.JoinHostPort(destination.String(), "1900") + "\r\nMAN: \"ssdp:discover\"\r\nMX: 1\r\nST: " + search + "\r\n\r\n"
			if _, err = conn.WriteToUDP([]byte(request), &net.UDPAddr{IP: destination, Port: 1900}); err != nil {
				return mailGateway{}, err
			}
		}
	}
	buffer := make([]byte, 8192)
	for {
		n, peer, err := conn.ReadFromUDP(buffer)
		if err != nil {
			return mailGateway{}, err
		}
		if !peer.IP.IsPrivate() || peer.IP.IsLoopback() || (len(upstream) == 0 && !sameMailSubnet(local, peer.IP)) || (len(upstream) > 0 && !peer.IP.Equal(upstream[0])) {
			continue
		}
		location := ""
		for _, line := range strings.Split(string(buffer[:n]), "\r\n") {
			if key, value, ok := strings.Cut(line, ":"); ok && strings.EqualFold(key, "location") {
				location = strings.TrimSpace(value)
			}
		}
		if !gatewayEndpoint(location, peer.IP) {
			continue
		}
		req, err := http.NewRequestWithContext(ctx, "GET", location, nil)
		if err != nil {
			continue
		}
		resp, err := gatewayClient().Do(req)
		if err != nil {
			continue
		}
		var description igdDescription
		err = xml.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&description)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 {
			continue
		}
		base, _ := url.Parse(location)
		if gatewayEndpoint(description.URLBase, peer.IP) {
			base, _ = url.Parse(description.URLBase)
		}
		var find func(igdDevice) (mailGateway, bool)
		find = func(device igdDevice) (mailGateway, bool) {
			for _, service := range device.Services {
				if service.Type != "urn:schemas-upnp-org:service:WANIPConnection:1" && service.Type != "urn:schemas-upnp-org:service:WANIPConnection:2" && service.Type != "urn:schemas-upnp-org:service:WANPPPConnection:1" {
					continue
				}
				relative, err := url.Parse(service.Control)
				if err != nil {
					continue
				}
				control := base.ResolveReference(relative).String()
				if gatewayEndpoint(control, peer.IP) {
					return mailGateway{service.Type, control}, true
				}
			}
			for _, child := range device.Devices {
				if result, ok := find(child); ok {
					return result, true
				}
			}
			return mailGateway{}, false
		}
		if result, ok := find(description.Device); ok {
			return result, nil
		}
	}
}

func sameMailSubnet(local, peer net.IP) bool {
	addresses, _ := net.InterfaceAddrs()
	for _, addr := range addresses {
		if network, ok := addr.(*net.IPNet); ok && network.IP.Equal(local) {
			return network.Contains(peer)
		}
	}
	return false
}

func gatewayEndpoint(raw string, peer net.IP) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Fragment != "" {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	return ip != nil && ip.IsPrivate() && !ip.IsLoopback() && ip.Equal(peer)
}

func gatewayClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

type gatewayError struct{ Code string }

func (e *gatewayError) Error() string {
	return "Router rejected the SMTP mapping (UPnP " + e.Code + ")"
}

func (g mailGateway) action(ctx context.Context, name string, args map[string]string) (map[string]string, error) {
	var body bytes.Buffer
	body.WriteString(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body><u:` + name + ` xmlns:u="` + g.Service + `">`)
	for _, key := range []string{"NewRemoteHost", "NewExternalPort", "NewProtocol", "NewInternalPort", "NewInternalClient", "NewEnabled", "NewPortMappingDescription", "NewLeaseDuration"} {
		if value, ok := args[key]; ok {
			body.WriteString("<" + key + ">")
			_ = xml.EscapeText(&body, []byte(value))
			body.WriteString("</" + key + ">")
		}
	}
	body.WriteString("</u:" + name + "></s:Body></s:Envelope>")
	req, err := http.NewRequestWithContext(ctx, "POST", g.Control, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", `"`+g.Service+"#"+name+`"`)
	resp, err := gatewayClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	decoder := xml.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	fields := map[string]string{}
	current := ""
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			current = value.Name.Local
		case xml.CharData:
			if text := strings.TrimSpace(string(value)); text != "" {
				fields[current] += text
			}
		}
	}
	if code := fields["errorCode"]; code != "" {
		return nil, &gatewayError{code}
	}
	if resp.StatusCode != 200 {
		return nil, errors.New("Router returned an unexpected HTTP response")
	}
	return fields, nil
}

func (g mailGateway) ensureSMTP(ctx context.Context, local string, owner ...string) (string, error) {
	description := smtpMappingDescription
	if len(owner) > 0 {
		description = owner[0]
	}
	query := map[string]string{"NewRemoteHost": "", "NewExternalPort": "25", "NewProtocol": "TCP"}
	existing, err := g.action(ctx, "GetSpecificPortMappingEntry", query)
	if err == nil {
		if existing["NewInternalClient"] != local || existing["NewInternalPort"] != "25" {
			return "", &inboundRouteIssue{Code: "other_mail_server", Message: "Incoming email already goes to another PC. Its route was preserved.", Router: routerHost(g.Control), Target: existing["NewInternalClient"]}
		}
		if existing["NewEnabled"] != "1" {
			return "", errors.New("An existing TCP25 router rule is disabled. Review it in the router before receiving mail.")
		}
		if existing["NewPortMappingDescription"] != description && existing["NewPortMappingDescription"] != smtpMappingDescription {
			return "Existing router forwarding reaches this PC on TCP25. Its settings were preserved. Internet delivery still needs verification.", nil
		}
		// Some routers acknowledge a renewal but retain the old description.
		// Migrate only our exact legacy target, then read the new rule back.
		if description != smtpMappingDescription && existing["NewPortMappingDescription"] == smtpMappingDescription {
			if _, err = g.action(ctx, "DeletePortMapping", query); err != nil {
				return "", errors.New("The existing Mail rule could not be upgraded. Review router SMTP forwarding and retry.")
			}
		}
	} else {
		var fault *gatewayError
		if !errors.As(err, &fault) || fault.Code != "714" {
			return "", errors.New("Router SMTP forwarding could not be checked. Forward public TCP25 to this PC, or configure an inbound mail gateway.")
		}
	}
	args := map[string]string{"NewRemoteHost": "", "NewExternalPort": "25", "NewProtocol": "TCP", "NewInternalPort": "25", "NewInternalClient": local, "NewEnabled": "1", "NewPortMappingDescription": description, "NewLeaseDuration": "1800"}
	for attempt := 0; attempt < 3; attempt++ {
		if _, err = g.action(ctx, "AddPortMapping", args); err == nil {
			break
		}
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
	if err != nil {
		endpoint, _ := url.Parse(g.Control)
		return "", fmt.Errorf("Router %s could not renew TCP25 (%v). Setup retries automatically. Review that router's forwarding settings or use an inbound mail gateway", endpoint.Hostname(), err)
	}
	check, err := g.action(ctx, "GetSpecificPortMappingEntry", query)
	if err != nil || check["NewInternalClient"] != local || check["NewInternalPort"] != "25" || check["NewEnabled"] != "1" || check["NewPortMappingDescription"] != description {
		return "", errors.New("Router forwarding was not confirmed. Review the TCP25 rule before expecting incoming mail.")
	}
	return "Router TCP25 forwarding is configured and renewed automatically. Gmail and other senders can retry pending mail; successful inbox delivery is checked separately.", nil
}

func routerHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
