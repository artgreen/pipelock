// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"strings"
	"testing"
)

func TestValidateBytes_AcceptsGoodConfig(t *testing.T) {
	if _, err := ValidateBytes([]byte("mode: balanced\n")); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestValidateBytes_RejectsUnknownField(t *testing.T) {
	_, err := ValidateBytes([]byte("mode: balanced\nbogus_field: true\n"))
	if err == nil {
		t.Fatal("expected unknown field to be rejected")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "bogus_field") {
		t.Errorf("error should name the offending field, got: %v", err)
	}
}

func TestValidateBytes_RejectsMultipleDocuments(t *testing.T) {
	_, err := ValidateBytes([]byte("mode: balanced\n---\nmode: audit\n"))
	if err == nil {
		t.Fatal("expected multi-document config to be rejected")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "multiple yaml documents") {
		t.Errorf("error should mention multiple documents, got: %v", err)
	}
}

// TestValidateBytes_SecurityDefaultFailClosed is the regression for the
// fail-open drift bug: response_scanning is force-enabled by
// applySecurityDefaults when `enabled` is omitted, so its action validator
// must run and reject an invalid action. A path that only ran ApplyDefaults
// would leave Enabled=false and wrongly report this config OK.
func TestValidateBytes_SecurityDefaultFailClosed(t *testing.T) {
	raw := "mode: balanced\nresponse_scanning:\n  action: invalid_action\n"
	_, err := ValidateBytes([]byte(raw))
	if err == nil {
		t.Fatal("expected omitted-enabled response_scanning with bad action to be rejected (fail-closed)")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "response_scanning") {
		t.Errorf("error should name response_scanning, got: %v", err)
	}
}
