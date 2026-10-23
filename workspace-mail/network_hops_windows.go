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
	"unsafe"

	"golang.org/x/sys/windows"
)

// Read the gateway of the adapter carrying this connection. This avoids
// guessing an address and works when a VPN or a second adapter is present.
func mailDefaultGateway(local net.IP) net.IP {
	size := uint32(16384)
	for n := 0; n < 3 && size <= 1<<20; n++ {
		buffer := make([]byte, size)
		first := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buffer[0]))
		err := windows.GetAdaptersAddresses(windows.AF_INET, windows.GAA_FLAG_INCLUDE_GATEWAYS, 0, first, &size)
		if err == windows.ERROR_BUFFER_OVERFLOW {
			continue
		}
		if err != nil {
			return nil
		}
		for adapter := first; adapter != nil; adapter = adapter.Next {
			matched := false
			for addr := adapter.FirstUnicastAddress; addr != nil; addr = addr.Next {
				if addr.Address.Sockaddr != nil && addr.Address.IP().Equal(local) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			for gateway := adapter.FirstGatewayAddress; gateway != nil; gateway = gateway.Next {
				if gateway.Address.Sockaddr != nil {
					ip := gateway.Address.IP()
					if ip.To4() != nil && ip.IsPrivate() && !ip.IsLoopback() {
						return append(net.IP(nil), ip...)
					}
				}
			}
		}
		return nil
	}
	return nil
}

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
