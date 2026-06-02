// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package pipelockclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stats" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"uptime_seconds":123,"requests":{"total":10,"allowed":7,"blocked":3,"block_rate":0.3},"sessions":{"active":2}}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	stats, err := c.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.Requests.Blocked != 3 || stats.Sessions.Active != 2 {
		t.Errorf("unexpected stats: %+v", stats)
	}
}
