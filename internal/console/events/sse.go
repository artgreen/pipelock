// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package events

import "sync"

// Hub fans out events to subscribed SSE clients.
type Hub struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

// NewHub creates an empty Hub.
func NewHub() *Hub { return &Hub{subs: make(map[chan Event]struct{})} }

// Subscribe registers a new client channel (buffered).
func (h *Hub) Subscribe() <-chan Event {
	ch := make(chan Event, 64)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes and closes a client channel.
func (h *Hub) Unsubscribe(ch <-chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.subs {
		if c == ch {
			delete(h.subs, c)
			close(c)
			return
		}
	}
}

// Broadcast sends to all subscribers, dropping for any that are full.
func (h *Hub) Broadcast(e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.subs {
		select {
		case c <- e:
		default: // slow consumer — drop rather than block
		}
	}
}
