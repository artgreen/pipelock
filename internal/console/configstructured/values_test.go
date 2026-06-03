// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package configstructured

import "testing"

func TestEffectiveValuesAndPresent(t *testing.T) {
	const raw = "mode: audit\nkill_switch:\n  api_token: \"super-secret\"\n"
	eff, err := EffectiveValues([]byte(raw), []string{"kill_switch.api_token"})
	if err != nil {
		t.Fatal(err)
	}
	if eff["mode"] != "audit" {
		t.Errorf("mode = %v", eff["mode"])
	}
	ks, _ := eff["kill_switch"].(map[string]any)
	if ks == nil {
		t.Fatalf("kill_switch not a map: %v", eff["kill_switch"])
	}
	if ks["api_token"] == "super-secret" {
		t.Error("api_token not redacted")
	}
	if ks["api_token"] != RedactedSentinel {
		t.Errorf("want redacted sentinel, got %v", ks["api_token"])
	}

	present := PresentPaths([]byte(raw))
	if !present["mode"] || !present["kill_switch.api_token"] {
		t.Errorf("present paths wrong: %v", present)
	}
	if present["fetch_proxy.listen"] {
		t.Error("absent path reported present")
	}
}

func TestEffectiveValuesAppliesDefaults(t *testing.T) {
	// mode omitted -> ApplyDefaults fills it (pipelock default is "balanced").
	eff, err := EffectiveValues([]byte("version: 1\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if eff["mode"] == nil || eff["mode"] == "" {
		t.Errorf("expected a defaulted mode, got %v", eff["mode"])
	}
}
