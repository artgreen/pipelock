// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package pipelockclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetSessions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"sessions":[{"key":"s1"},{"key":"s2"}],"count":2}`))
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL})
	got, err := c.GetSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Count != 2 || len(got.Sessions) != 2 {
		t.Errorf("unexpected sessions: %+v", got)
	}
}

func TestGetHealthReportsDownWhenUnreachable(t *testing.T) {
	c := New(Options{BaseURL: "http://127.0.0.1:1"}) // nothing listening
	if c.Healthy(context.Background()) {
		t.Error("expected Healthy=false for unreachable pipelock")
	}
}

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

func TestKillSwitchToggleSendsBearer(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{"active":true,"source":"api"}`))
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL, APIToken: "secret123"})
	if err := c.SetKillSwitch(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer secret123" {
		t.Errorf("missing/incorrect bearer: %q", gotAuth)
	}
	if !strings.Contains(gotBody, `"active":true`) {
		t.Errorf("unexpected body: %q", gotBody)
	}
}

func TestGetKillSwitchStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/killswitch/status" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"active":true,"sources":{"api":true,"config":false},"message":"locked"}`))
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL, APIToken: "tok"})
	st, err := c.GetKillSwitch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !st.Active || !st.Sources["api"] || st.Message != "locked" {
		t.Errorf("unexpected status: %+v", st)
	}
}
