// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package configintents

import (
	"strings"
	"testing"

	"github.com/luckyPipewrench/pipelock/internal/blockreason"
)

func TestProposeUnblock(t *testing.T) {
	tests := []struct {
		name           string
		target         string
		reason         string
		matchedPattern string
		wantOp         string
		wantPath       string
		wantValue      string
		wantWarn       bool
		wantErr        bool
		wantScanned    string // substring at least one StillScanned entry must contain
	}{
		{"ssrf ipv4 -> /32", "10.1.2.3", string(blockreason.SSRFPrivateIP), "", OpListAdd, "ssrf.ip_allowlist", "10.1.2.3/32", false, false, "DLP"},
		{"ssrf url -> host /32", "http://10.1.2.3:9000/x", string(blockreason.SSRFPrivateIP), "", OpListAdd, "ssrf.ip_allowlist", "10.1.2.3/32", false, false, "DLP"},
		{"ssrf hostport -> /32", "10.1.2.3:9000", string(blockreason.SSRFPrivateIP), "", OpListAdd, "ssrf.ip_allowlist", "10.1.2.3/32", false, false, "DLP"},
		{"ssrf ipv6 -> /128", "fd00::5", string(blockreason.SSRFPrivateIP), "", OpListAdd, "ssrf.ip_allowlist", "fd00::5/128", false, false, "DLP"},
		{"ssrf existing cidr kept", "10.0.0.0/8", string(blockreason.SSRFPrivateIP), "", OpListAdd, "ssrf.ip_allowlist", "10.0.0.0/8", false, false, "DLP"},
		{"ssrf non-canonical cidr masked", "10.5.0.0/8", string(blockreason.SSRFPrivateIP), "", OpListAdd, "ssrf.ip_allowlist", "10.0.0.0/8", false, false, "DLP"},
		{"ssrf non-ip host errors", "internal.local", string(blockreason.SSRFPrivateIP), "", "", "", "", false, true, ""},
		{"metadata warns", "169.254.169.254", string(blockreason.SSRFMetadata), "", OpListAdd, "ssrf.ip_allowlist", "169.254.169.254/32", true, false, "DLP"},
		{"blocklist remove host (no pattern, warns)", "http://x.pastebin.com/raw", string(blockreason.DomainBlocklist), "", OpListRemove, "fetch_proxy.monitoring.blocklist", "x.pastebin.com", true, false, "SSRF"},
		{"blocklist remove matched wildcard", "http://x.pastebin.com/raw", string(blockreason.DomainBlocklist), "*.pastebin.com", OpListRemove, "fetch_proxy.monitoring.blocklist", "*.pastebin.com", false, false, "SSRF"},
		{"dns rebind refused", "1.2.3.4", string(blockreason.SSRFDNSRebind), "", "", "", "", false, true, ""},
		{"unknown reason errors", "1.2.3.4", "totally_unknown", "", "", "", "", false, true, ""},
		{"empty target errors", "", string(blockreason.SSRFPrivateIP), "", "", "", "", false, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ProposeUnblock(tt.target, tt.reason, tt.matchedPattern)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got proposal %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Op != tt.wantOp || got.Path != tt.wantPath || got.Value != tt.wantValue {
				t.Errorf("got {op:%q path:%q value:%q}, want {op:%q path:%q value:%q}", got.Op, got.Path, got.Value, tt.wantOp, tt.wantPath, tt.wantValue)
			}
			if (got.Warning != "") != tt.wantWarn {
				t.Errorf("warning presence = %v, want %v (warning=%q)", got.Warning != "", tt.wantWarn, got.Warning)
			}
			if strings.TrimSpace(got.Explanation) == "" {
				t.Error("explanation must not be empty")
			}
			if len(got.StillScanned) == 0 {
				t.Error("still_scanned must list remaining protections")
			}
			if tt.wantScanned != "" {
				found := false
				for _, s := range got.StillScanned {
					if strings.Contains(s, tt.wantScanned) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("still_scanned %v does not contain entry with %q", got.StillScanned, tt.wantScanned)
				}
			}
		})
	}
}
