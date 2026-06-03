// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package configintents maps a blocked destination to the minimal, safe config
// change that would permit it. It only computes proposals; applying them goes
// through the console's existing validate->apply path.
package configintents

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/luckyPipewrench/pipelock/internal/blockreason"
)

// Op codes for a Proposal.
const (
	OpListAdd    = "list_add"
	OpListRemove = "list_remove"
)

// Config paths the unblock recipe can touch.
const (
	PathSSRFIPAllowlist = "ssrf.ip_allowlist"
	PathBlocklist       = "fetch_proxy.monitoring.blocklist"
)

// Proposal is a minimal config change that allows a previously-blocked
// destination. It is computed, never applied here.
type Proposal struct {
	Op           string   `json:"op"`            // list_add | list_remove
	Path         string   `json:"path"`          // dotted config path
	Value        string   `json:"value"`         // item to add/remove
	Explanation  string   `json:"explanation"`   // plain-language description
	StillScanned []string `json:"still_scanned"` // protections that remain active
	Warning      string   `json:"warning,omitempty"`
}

// ProposeUnblock maps a (target, reason) to the minimal config change that
// permits the destination. Fails closed: unknown/unsupported reasons return an
// error rather than guessing.
//
// matchedPattern is the blocklist entry that actually matched, when known
// (e.g. a wildcard "*.pastebin.com"). It is used only by the DomainBlocklist
// case so the proposal removes the entry that is really blocking the host
// rather than the exact requested host, which may not be present verbatim.
func ProposeUnblock(target, reason, matchedPattern string) (Proposal, error) {
	host := normalizeTarget(target)
	if host == "" {
		return Proposal{}, fmt.Errorf("empty or unparseable target %q", target)
	}
	switch blockreason.Reason(reason) {
	case blockreason.SSRFPrivateIP:
		cidr, err := hostToCIDR(host)
		if err != nil {
			return Proposal{}, err
		}
		return Proposal{
			Op:          OpListAdd,
			Path:        PathSSRFIPAllowlist,
			Value:       cidr,
			Explanation: fmt.Sprintf("Adds %s to ssrf.ip_allowlist, exempting only this address from the private-IP / SSRF block.", cidr),
			StillScanned: []string{
				"DLP secret scanning (runs before the SSRF layer)",
				"prompt-injection / response scanning",
				"domain blocklist",
			},
		}, nil
	case blockreason.SSRFMetadata:
		cidr, err := hostToCIDR(host)
		if err != nil {
			return Proposal{}, err
		}
		return Proposal{
			Op:           OpListAdd,
			Path:         PathSSRFIPAllowlist,
			Value:        cidr,
			Explanation:  fmt.Sprintf("Adds %s to ssrf.ip_allowlist. This address is a cloud instance-metadata endpoint.", cidr),
			StillScanned: []string{"DLP secret scanning", "prompt-injection / response scanning"},
			Warning: "This is a cloud instance-metadata address (e.g. 169.254.169.254). Allowing it is a common SSRF " +
				"escalation path and can expose cloud credentials. Only proceed if you specifically intend to allow metadata access.",
		}, nil
	case blockreason.DomainBlocklist:
		value := host
		warning := ""
		if matchedPattern != "" {
			value = matchedPattern
		} else {
			warning = "If the blocklist entry is a wildcard (e.g. *.pastebin.com), removing the exact host may not unblock it — " +
				"use the blocklist editor to remove the matching pattern instead."
		}
		return Proposal{
			Op:          OpListRemove,
			Path:        PathBlocklist,
			Value:       value,
			Explanation: fmt.Sprintf("Removes %s from fetch_proxy.monitoring.blocklist so %s is no longer blocked at the domain layer.", value, host),
			StillScanned: []string{
				"DLP secret scanning",
				"prompt-injection / response scanning",
				"SSRF / private-IP checks",
			},
			Warning: warning,
		}, nil
	case blockreason.SSRFDNSRebind:
		return Proposal{}, fmt.Errorf("reason %q has no safe minimal allow: the host resolved to a private IP after appearing public; allowing it would defeat the rebinding protection", reason)
	default:
		return Proposal{}, fmt.Errorf("unsupported block reason %q for one-click unblock", reason)
	}
}

// normalizeTarget extracts a host or IP from a URL, host:port, or bare host.
func normalizeTarget(target string) string {
	t := strings.TrimSpace(target)
	if t == "" {
		return ""
	}
	if strings.Contains(t, "://") {
		if u, err := url.Parse(t); err == nil && u.Hostname() != "" {
			return u.Hostname()
		}
	}
	if h, _, err := net.SplitHostPort(t); err == nil && h != "" {
		return h
	}
	return t
}

// hostToCIDR turns a bare IP into a single-host CIDR; canonicalizes an existing
// CIDR (so host bits are masked off and config.Validate accepts it); errors on a
// non-IP host (SSRF blocks are on resolved IPs).
func hostToCIDR(host string) (string, error) {
	if _, ipNet, err := net.ParseCIDR(host); err == nil {
		return ipNet.String(), nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return "", fmt.Errorf("SSRF allow needs an IP address, got %q", host)
	}
	if ip.To4() != nil {
		return ip.String() + "/32", nil
	}
	return ip.String() + "/128", nil
}
