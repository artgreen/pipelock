// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package configstructured

import (
	"strings"
	"testing"
)

func TestApplyChanges(t *testing.T) {
	raw := []byte("mode: audit\nfetch_proxy:\n  listen: \"127.0.0.1:8888\"\n")
	help := func(path string) string {
		if path == "metrics_listen" {
			return "Separate listen address for /metrics and /stats."
		}
		return ""
	}
	out, err := ApplyChanges(raw, map[string]any{
		"mode":           "strict",
		"metrics_listen": "127.0.0.1:9095",
	}, help)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "mode: strict") || !strings.Contains(s, "metrics_listen: 127.0.0.1:9095") {
		t.Errorf("changes not applied:\n%s", s)
	}
	if !strings.Contains(s, "Separate listen address") {
		t.Errorf("help comment missing on new field:\n%s", s)
	}
	// 2-space indentation (pipelock convention), not yaml.v3 default 4.
	if strings.Contains(s, "    listen:") {
		t.Errorf("expected 2-space indent, got 4:\n%s", s)
	}
}

func TestApplyChangesNilDeletes(t *testing.T) {
	raw := []byte("mode: audit\nmetrics_listen: \"127.0.0.1:9095\"\n")
	out, err := ApplyChanges(raw, map[string]any{"metrics_listen": nil}, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "metrics_listen") {
		t.Errorf("nil value should delete the key:\n%s", out)
	}
}

func TestApplyChangesSkipsRedactedSecret(t *testing.T) {
	raw := []byte("kill_switch:\n  api_token: \"real-token\"\n")
	out, err := ApplyChanges(raw, map[string]any{"kill_switch.api_token": RedactedSentinel}, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	// Leaving the masked sentinel must NOT overwrite the real secret.
	if !strings.Contains(string(out), "real-token") {
		t.Errorf("redacted sentinel overwrote the secret:\n%s", out)
	}
}
