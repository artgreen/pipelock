// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIngestStoresAndBroadcasts(t *testing.T) {
	buf := NewBuffer(10)
	hub := NewHub()
	ch := hub.Subscribe()
	h := IngestHandler(buf, hub)

	body := `{"severity":"critical","type":"dlp.secret","fields":{"target":"api.openai.com"}}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/ingest", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := buf.Snapshot(); len(got) != 1 || got[0].Type != "dlp.secret" {
		t.Errorf("event not buffered: %+v", got)
	}
	select {
	case e := <-ch:
		if e.Type != "dlp.secret" {
			t.Errorf("broadcast type = %q", e.Type)
		}
	default:
		t.Error("event not broadcast")
	}
}

func TestIngestRejectsNonPOST(t *testing.T) {
	h := IngestHandler(NewBuffer(1), NewHub())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ingest", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d", rec.Code)
	}
}
