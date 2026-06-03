// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"encoding/json"
	"net/http"
)

const maxIngestBody = 64 * 1024

// IngestHandler returns an http.Handler that accepts pipelock webhook events,
// stores them in buf, and broadcasts them to hub subscribers.
func IngestHandler(buf *Buffer, hub *Hub) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxIngestBody)
		var e Event
		if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
			http.Error(w, "invalid event", http.StatusBadRequest)
			return
		}
		buf.Add(e)
		hub.Broadcast(e)
		w.WriteHeader(http.StatusNoContent)
	})
}
