// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package configstructured

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/luckyPipewrench/pipelock/internal/config"
)

// Editing one field must never introduce tri-state security defaults (enforce,
// internal, scan_content) that were absent from the original file — otherwise a
// nil-means-default would be silently pinned to an explicit value.
func TestEditDoesNotLeakTriStateDefaults(t *testing.T) {
	const raw = "mode: balanced\nfetch_proxy:\n  monitoring:\n    blocklist:\n      - \"*.pastebin.com\"\n"
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatal(err)
	}
	if err := ApplyChange(&doc, "mode", "strict", ""); err != nil {
		t.Fatal(err)
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"enforce:", "internal:", "scan_content:"} {
		if strings.Contains(string(out), k) {
			t.Errorf("edit leaked %q into config:\n%s", k, out)
		}
	}
	// The pre-existing blocklist must survive.
	if !strings.Contains(string(out), "*.pastebin.com") {
		t.Errorf("edit dropped existing content:\n%s", out)
	}
}

// A no-edit round-trip must not introduce any of the tri-state keys either.
func TestNoEditRoundTripKeepsOmissions(t *testing.T) {
	const raw = "mode: audit\n"
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatal(err)
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "enforce") || strings.Contains(string(out), "internal") {
		t.Errorf("round-trip expanded omitted fields:\n%s", out)
	}
}

// Whatever the editor writes must pass pipelock's real validator (validate-before-write).
func TestEditedConfigStillValidates(t *testing.T) {
	const raw = "mode: audit\n"
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatal(err)
	}
	if err := ApplyChange(&doc, "metrics_listen", "127.0.0.1:9095", ""); err != nil {
		t.Fatal(err)
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.ValidateBytes(out); err != nil {
		t.Fatalf("patched config failed validation: %v\n%s", err, out)
	}
}
