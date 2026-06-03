// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package events ingests pipelock's emitted events and fans them out to
// browser clients over SSE.
package events

import "sync"

// Event mirrors pipelock's webhook payload (internal/emit/webhook.go).
type Event struct {
	Severity  string         `json:"severity"`
	Type      string         `json:"type"`
	Timestamp string         `json:"timestamp"`
	Instance  string         `json:"pipelock_instance"`
	Fields    map[string]any `json:"fields"`
}

// Buffer is a fixed-capacity ring of recent events, safe for concurrent use.
type Buffer struct {
	mu    sync.Mutex
	items []Event
	cap   int
}

// NewBuffer creates a ring buffer retaining the most recent capacity events.
func NewBuffer(capacity int) *Buffer {
	if capacity < 1 {
		capacity = 1
	}
	return &Buffer{cap: capacity}
}

// Add appends an event, evicting the oldest when at capacity.
func (b *Buffer) Add(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.items = append(b.items, e)
	if len(b.items) > b.cap {
		b.items = b.items[len(b.items)-b.cap:]
	}
}

// Snapshot returns a copy of retained events, oldest first.
func (b *Buffer) Snapshot() []Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Event, len(b.items))
	copy(out, b.items)
	return out
}
