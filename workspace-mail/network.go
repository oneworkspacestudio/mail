// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"bytes"
	"context"
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
	Device igdDevice `xml:"device"`
}

func (a *App) inboundProgress(state, message string) {
	a.mu.Lock()
	a.inboundState = state
	a.inboundMessage = message
	a.mu.Unlock()
}

// DNS and an HTTPS tunnel do not open SMTP through NAT. Maintain a leased,
// conflict-checked mapping only for the managed engine's public TCP25 listener.
func (a *App) maintainInboundMail(ctx context.Context) {
	for {
		a.mu.RLock()
		c := a.cfg
		a.mu.RUnlock()
		delay := 5 * time.Minute
		if !c.Configured {
			a.inboundProgress("checking", "Waiting for the mail server to finish setup.")
			delay = 5 * time.Second
		} else {
			a.inboundProgress("checking", "Checking router forwarding and public TCP25 reachability…")
			attempt, cancel := context.WithTimeout(ctx, 40*time.Second)
			routeMessage, routeErr := configureInboundRoute(attempt, c.EngineHostname)
			state, probeMessage := checkIncomingSMTP(attempt, c.EngineHostname)
			cancel()
			if state == "reachable" && routeErr == nil {
				a.inboundProgress(state, probeMessage+" "+routeMessage)
			} else if routeErr != nil {
				if state == "reachable" {
					state = "unverified"
					probeMessage = "Public TCP25 responds, but routing to this PC has not been confirmed. Review the router rule before expecting delivery."
				}
				a.inboundProgress(state, routeErr.Error()+" "+probeMessage)
			} else {
				a.inboundProgress(state, routeMessage+" "+probeMessage)
			}
		}
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
func checkIncomingSMTP(ctx context.Context, hostname string) (string, string) {
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
	var started struct {
		ID string `json:"request_id"`
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

func configureInboundRoute(ctx context.Context, hostname string) (string, error) {
	connection, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(1, 1, 1, 1), Port: 53})
	if err != nil {
		return "", errors.New("Incoming mail needs a network connection. Setup will retry automatically.")
	}
	local := connection.LocalAddr().(*net.UDPAddr).IP
	connection.Close()
	if !local.IsPrivate() {
		return "This PC has a public network address. Incoming email still requires TCP25 through its firewall; HTTPS alone does not verify delivery.", nil
	}
	gateway, err := discoverMailGateway(ctx, local)
	if err != nil {
		return "", fmt.Errorf("Incoming email cannot use the web tunnel. Enable router UPnP or forward public TCP25 to %s:25, and check your ISP permits incoming SMTP. Setup retries automatically.", local)
	}
	fields, err := gateway.action(ctx, "GetExternalIPAddress", nil)
	if err != nil {
		return "", errors.New("The router could not report its Internet address. Forward TCP25 to this Windows PC or configure an inbound mail gateway.")
	}
	external := net.ParseIP(fields["NewExternalIPAddress"])
	if !publicMailIP(external) {
		return "", errors.New("The router is behind another router or shared ISP NAT. Incoming mail needs forwarding through the upstream router, a public IP, or an inbound mail gateway.")
	}
	addresses, err := net.DefaultResolver.LookupIP(ctx, "ip4", hostname)
	if err != nil {
		return "", errors.New("Waiting for the SMTP hostname's public DNS address. Network setup will retry automatically.")
	}
	match := false
	for _, ip := range addresses {
		if ip.Equal(external) {
			match = true
		}
	}
	if !match {
		return "", errors.New("The router's public IP does not match the SMTP DNS address. Review the hosting DNS/IP before opening incoming mail.")
	}
	return gateway.ensureSMTP(ctx, local.String())
}

func discoverMailGateway(ctx context.Context, local net.IP) (mailGateway, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: local})
	if err != nil {
		return mailGateway{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	request := "M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 1\r\nST: urn:schemas-upnp-org:device:InternetGatewayDevice:1\r\n\r\n"
	if _, err = conn.WriteToUDP([]byte(request), &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}); err != nil {
		return mailGateway{}, err
	}
	buffer := make([]byte, 8192)
	for {
		n, peer, err := conn.ReadFromUDP(buffer)
		if err != nil {
			return mailGateway{}, err
		}
		if !peer.IP.IsPrivate() || peer.IP.IsLoopback() || !sameMailSubnet(local, peer.IP) {
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

func (g mailGateway) ensureSMTP(ctx context.Context, local string) (string, error) {
	query := map[string]string{"NewRemoteHost": "", "NewExternalPort": "25", "NewProtocol": "TCP"}
	existing, err := g.action(ctx, "GetSpecificPortMappingEntry", query)
	if err == nil {
		if existing["NewInternalClient"] != local || existing["NewInternalPort"] != "25" {
			return "", errors.New("Router TCP25 already belongs to another server. Its route was preserved; migrate it explicitly or use an inbound mail gateway.")
		}
		if existing["NewEnabled"] != "1" {
			return "", errors.New("An existing TCP25 router rule is disabled. Review it in the router before receiving mail.")
		}
		if existing["NewPortMappingDescription"] != smtpMappingDescription {
			return "Existing router forwarding reaches this PC on TCP25. Its settings were preserved. Internet delivery still needs verification.", nil
		}
	} else {
		var fault *gatewayError
		if !errors.As(err, &fault) || fault.Code != "714" {
			return "", errors.New("Router SMTP forwarding could not be checked. Forward public TCP25 to this PC, or configure an inbound mail gateway.")
		}
	}
	args := map[string]string{"NewRemoteHost": "", "NewExternalPort": "25", "NewProtocol": "TCP", "NewInternalPort": "25", "NewInternalClient": local, "NewEnabled": "1", "NewPortMappingDescription": smtpMappingDescription, "NewLeaseDuration": "1800"}
	if _, err = g.action(ctx, "AddPortMapping", args); err != nil {
		return "", errors.New("The router could not open TCP25 automatically. Forward TCP25 to this PC in the router, check your ISP's SMTP policy, or use an inbound mail gateway.")
	}
	check, err := g.action(ctx, "GetSpecificPortMappingEntry", query)
	if err != nil || check["NewInternalClient"] != local || check["NewInternalPort"] != "25" || check["NewEnabled"] != "1" || check["NewPortMappingDescription"] != smtpMappingDescription {
		return "", errors.New("Router forwarding was not confirmed. Review the TCP25 rule before expecting incoming mail.")
	}
	return "Router TCP25 forwarding is configured and renewed automatically. Gmail and other senders can retry pending mail; successful inbox delivery is checked separately.", nil
}
