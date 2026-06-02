// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package configsvc

import (
	"strings"
	"testing"
)

func TestValidateAcceptsGoodConfig(t *testing.T) {
	good := "mode: balanced\n"
	res := Validate([]byte(good))
	if !res.OK {
		t.Fatalf("expected valid, got error: %s", res.Error)
	}
}

func TestValidateRejectsUnknownField(t *testing.T) {
	bad := "mode: balanced\nbogus_field: true\n"
	res := Validate([]byte(bad))
	if res.OK {
		t.Fatal("expected invalid config to be rejected")
	}
	if !strings.Contains(strings.ToLower(res.Error), "bogus_field") {
		t.Errorf("error should name the offending field, got: %s", res.Error)
	}
}

// TestValidateRejectsFailOpenSecurityDefault is the regression for the
// fail-open drift bug: response_scanning is force-enabled (fail-closed) when
// `enabled` is omitted, so an invalid action must be rejected. A path that
// skipped applySecurityDefaults would wrongly report this config OK.
func TestValidateRejectsFailOpenSecurityDefault(t *testing.T) {
	bad := "mode: balanced\nresponse_scanning:\n  action: invalid_action\n"
	res := Validate([]byte(bad))
	if res.OK {
		t.Fatal("expected omitted-enabled response_scanning with bad action to be rejected (fail-closed)")
	}
	if !strings.Contains(strings.ToLower(res.Error), "response_scanning") {
		t.Errorf("error should name response_scanning, got: %s", res.Error)
	}
}
