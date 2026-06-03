// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"net/http"

	"github.com/luckyPipewrench/pipelock/internal/console/events"
)

func serveSSE(w http.ResponseWriter, r *http.Request, hub *events.Hub, buf *events.Buffer) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Subscribe before Snapshot so no event is missed; an event in the small
	// window between the two may be delivered twice — clients dedupe by timestamp.
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	for _, e := range buf.Snapshot() {
		writeSSE(w, e)
	}
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(w, e)
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, e events.Event) {
	data, _ := json.Marshal(e)
	_, _ = w.Write([]byte("data: "))
	_, _ = w.Write(data)
	_, _ = w.Write([]byte("\n\n"))
}
