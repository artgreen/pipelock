// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package configstructured

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func mustPatch(t *testing.T, src, path string, val any, help string) string {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	if err := ApplyChange(&doc, path, val, help); err != nil {
		t.Fatalf("ApplyChange(%s): %v", path, err)
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestApplyChange(t *testing.T) {
	const src = "mode: audit\nfetch_proxy:\n  listen: \"127.0.0.1:8888\"   # keep\n"

	out := mustPatch(t, src, "mode", "strict", "")
	if !strings.Contains(out, "mode: strict") || !strings.Contains(out, "# keep") {
		t.Errorf("replace failed:\n%s", out)
	}

	out = mustPatch(t, src, "fetch_proxy.timeout_seconds", 30, "Per-request timeout.")
	if strings.Count(out, "fetch_proxy:") != 1 {
		t.Errorf("duplicate parent:\n%s", out)
	}
	if !strings.Contains(out, "timeout_seconds: 30") {
		t.Errorf("insert failed:\n%s", out)
	}
	if !strings.Contains(out, "Per-request timeout.") {
		t.Errorf("head comment missing:\n%s", out)
	}

	out = mustPatch(t, src, "kill_switch.api_listen", "127.0.0.1:9090", "")
	if !strings.Contains(out, "kill_switch:") || !strings.Contains(out, "api_listen: 127.0.0.1:9090") {
		t.Errorf("new section failed:\n%s", out)
	}

	out = mustPatch(t, src, "mode", DeleteSentinel, "")
	if strings.Contains(out, "mode:") {
		t.Errorf("delete failed:\n%s", out)
	}
}

func TestApplyChangeBool(t *testing.T) {
	out := mustPatch(t, "enforce: true\n", "enforce", false, "")
	if !strings.Contains(out, "enforce: false") {
		t.Errorf("bool replace:\n%s", out)
	}
}

func TestApplyChangeList(t *testing.T) {
	out := mustPatch(t, "api_allowlist:\n  - a.com\n", "api_allowlist", []any{"a.com", "b.com"}, "")
	if !strings.Contains(out, "b.com") {
		t.Errorf("list replace failed:\n%s", out)
	}
}

func TestApplyChangeRejectsScalarTraversal(t *testing.T) {
	var doc yaml.Node
	_ = yaml.Unmarshal([]byte("mode: audit\n"), &doc)
	if err := ApplyChange(&doc, "mode.sub.key", "x", ""); err == nil {
		t.Error("expected error traversing through a scalar, got nil")
	}
}
