// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package events

import "testing"

func TestRingBufferEvictsOldest(t *testing.T) {
	b := NewBuffer(3)
	for i := 1; i <= 5; i++ {
		b.Add(Event{Type: "t", Fields: map[string]any{"i": i}})
	}
	got := b.Snapshot()
	if len(got) != 3 {
		t.Fatalf("expected 3 retained, got %d", len(got))
	}
	if got[0].Fields["i"] != 3 || got[2].Fields["i"] != 5 {
		t.Errorf("expected oldest evicted, got %+v", got)
	}
}
