// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"sync"
)

var jwtPattern = regexp.MustCompile(`[A-Za-z0-9_-]{12,}\.[A-Za-z0-9_-]{12,}\.[A-Za-z0-9_-]{12,}`)

// Upstream authentication failures can include a raw access token. Keep that
// material out of the launcher's persistent log, even when authentication fails.
type engineLog struct {
	mu      sync.Mutex
	out     io.Writer
	pending []byte
	secrets []string
}

func (l *engineLog) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := len(b)
	l.pending = append(l.pending, b...)
	for {
		idx := bytes.IndexByte(l.pending, '\n')
		if idx < 0 {
			break
		}
		line := string(l.pending[:idx+1])
		l.pending = l.pending[idx+1:]
		if strings.Contains(line, "(auth.") && strings.Contains(line, "details") {
			line = "Stalwart authentication event (sensitive details redacted)\n"
		}
		line = jwtPattern.ReplaceAllString(line, "[redacted-token]")
		for _, s := range l.secrets {
			if s != "" {
				line = strings.ReplaceAll(line, s, "[redacted-secret]")
			}
		}
		if _, e := io.WriteString(l.out, line); e != nil {
			return n, e
		}
	}
	if len(l.pending) > 65536 {
		l.pending = nil
		_, _ = io.WriteString(l.out, "Stalwart oversized log entry omitted\n")
	}
	return n, nil
}
