// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package configschema

import "testing"

func TestLoadDescriptorEmbedded(t *testing.T) {
	d, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d == nil {
		t.Fatal("nil descriptor")
	}
	if got := d.Help("nope.nope"); got != "" {
		t.Errorf("Help(unknown) = %q, want empty", got)
	}
}
