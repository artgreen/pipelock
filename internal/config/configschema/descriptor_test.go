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

func TestDescriptorCoversWholeSchema(t *testing.T) {
	d, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	var leaves int
	var walk func(fs []Field)
	walk = func(fs []Field) {
		for i := range fs {
			if fs[i].Type == TypeGroup {
				walk(fs[i].Children)
			} else {
				leaves++
			}
		}
	}
	walk(d.Sections)
	if leaves != d.FieldCount || leaves == 0 {
		t.Fatalf("leaf count %d != FieldCount %d", leaves, d.FieldCount)
	}
	if d.Help("enforce") == "" {
		t.Error("enforce should have help text")
	}
}

func TestDescriptorSpotChecks(t *testing.T) {
	d, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	find := func(path string) *Field {
		var out *Field
		var walk func(fs []Field)
		walk = func(fs []Field) {
			for i := range fs {
				if fs[i].Path == path {
					out = &fs[i]
					return
				}
				walk(fs[i].Children)
			}
		}
		walk(d.Sections)
		return out
	}
	if f := find("mode"); f == nil || f.Type != TypeEnum || len(f.Enum) == 0 {
		t.Errorf("mode should be enum with values, got %+v", f)
	}
	if f := find("enforce"); f == nil || f.Type != TypeTriState {
		t.Errorf("enforce should be tristate, got %+v", f)
	}
	if f := find("fetch_proxy"); f == nil || f.Type != TypeGroup {
		t.Errorf("fetch_proxy should be a group, got %+v", f)
	}
	if f := find("fetch_proxy.monitoring.blocklist"); f == nil || f.Type != TypeList {
		t.Errorf("blocklist should be a list, got %+v", f)
	}
	if f := find("kill_switch.api_token"); f == nil || !f.Secret {
		t.Errorf("api_token should be secret, got %+v", f)
	}
}
