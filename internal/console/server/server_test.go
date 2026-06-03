// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luckyPipewrench/pipelock/internal/console/auth"
	"github.com/luckyPipewrench/pipelock/internal/console/configsvc"
	"github.com/luckyPipewrench/pipelock/internal/console/events"
	"github.com/luckyPipewrench/pipelock/internal/console/pipelockclient"
	"github.com/luckyPipewrench/pipelock/internal/console/service"
)

func newTestServer(t *testing.T, configPath, passwordHash string) http.Handler {
	t.Helper()
	mgr := auth.NewManager(auth.Options{PasswordHash: passwordHash, SecretHex: "00112233445566778899aabbccddeeff"})
	return New(Deps{
		Auth:    mgr,
		Config:  configsvc.New(configPath),
		Client:  pipelockclient.New(pipelockclient.Options{BaseURL: "http://127.0.0.1:1"}),
		Service: service.New("pipelock"),
		Buffer:  events.NewBuffer(100),
		Hub:     events.NewHub(),
	})
}

func TestConfigEndpointRequiresAuth(t *testing.T) {
	h := newTestServer(t, "/tmp/none.yaml", "$argon2id$x")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/config", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestProtectedRoutesRequireAuth(t *testing.T) {
	h := newTestServer(t, "/tmp/none.yaml", "$argon2id$x")
	cases := []struct{ method, path string }{
		{http.MethodGet, "/api/stats"},
		{http.MethodGet, "/api/sessions"},
		{http.MethodGet, "/api/killswitch"},
		{http.MethodPost, "/api/killswitch"},
		{http.MethodGet, "/api/config"},
		{http.MethodPost, "/api/config"},
		{http.MethodPost, "/api/config/validate"},
		{http.MethodGet, "/api/service"},
		{http.MethodPost, "/api/service/restart"},
		{http.MethodPost, "/api/logout"},
		{http.MethodGet, "/api/events"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), c.method, c.path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: got %d, want 401", c.method, c.path, rec.Code)
		}
	}
}

func TestSetupReportsNeedsSetup(t *testing.T) {
	h := newTestServer(t, "/tmp/none.yaml", "")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/setup", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "true") {
		t.Errorf("setup status: code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestConfigWriteAppliesValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/pipelock.yaml"
	_ = os.WriteFile(path, []byte("mode: audit\n"), 0o600)
	hash, _ := auth.HashPassword("pw")
	h := newTestServer(t, path, hash)

	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/login", strings.NewReader(`{"password":"pw"}`)))
	cookie := loginRec.Result().Cookies()[0]

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/config", strings.NewReader("mode: balanced\n"))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("write status = %d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := os.ReadFile(filepath.Clean(path))
	if string(got) != "mode: balanced\n" {
		t.Errorf("config not written: %q", got)
	}
}

func TestSetupRejectedWhenAlreadyConfigured(t *testing.T) {
	hash, _ := auth.HashPassword("pw")
	h := newTestServer(t, "/tmp/none.yaml", hash)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/setup", strings.NewReader(`{"password":"new"}`)))
	if rec.Code != http.StatusConflict {
		t.Errorf("setup when configured should 409, got %d", rec.Code)
	}
}
