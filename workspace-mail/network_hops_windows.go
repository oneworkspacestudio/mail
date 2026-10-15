// SPDX-License-Identifier: AGPL-3.0-only
//go:build windows

package main

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"
	"time"
)

// Some mesh gateways report the outer public IP even behind another NAT.
// Use bounded header-only ICMP route discovery, without router credentials.
func privateMailHops(ctx context.Context) []net.IP {
	system := os.Getenv("SystemRoot")
	if !filepath.IsAbs(system) {
		return nil
	}
	short, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(short, filepath.Join(system, "System32", "tracert.exe"), "-d", "-h", "3", "-w", "400", "1.1.1.1")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	output, _ := cmd.Output()
	return parsePrivateMailHops(string(output))
}
func parsePrivateMailHops(text string) []net.IP {
	var result []net.IP
	seen := map[string]bool{}
	for _, value := range regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`).FindAllString(text, -1) {
		ip := net.ParseIP(value)
		if ip != nil && ip.IsPrivate() && !ip.IsLoopback() && !seen[value] {
			seen[value] = true
			result = append(result, ip)
		}
	}
	return result
}
