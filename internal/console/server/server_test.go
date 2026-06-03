// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luckyPipewrench/pipelock/internal/console/auth"
	"github.com/luckyPipewrench/pipelock/internal/console/configintents"
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
		{http.MethodPost, "/api/config/unblock-proposal"},
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
	if rec.Code != http.StatusNoContent {
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

func TestUnblockProposalEndpoint(t *testing.T) {
	hash, _ := auth.HashPassword("pw")
	h := newTestServer(t, t.TempDir()+"/pipelock.yaml", hash)

	// Unauthenticated request must be 401.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/config/unblock-proposal", strings.NewReader(`{"target":"10.1.2.3","reason":"ssrf_private_ip"}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthed: got %d, want 401", rec.Code)
	}

	// Log in.
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/login", strings.NewReader(`{"password":"pw"}`)))
	cookies := loginRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie after login")
	}
	cookie := cookies[0]

	// Valid request → 200 with expected proposal.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/config/unblock-proposal", strings.NewReader(`{"target":"10.1.2.3","reason":"ssrf_private_ip"}`))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid proposal: got %d body=%s", rec.Code, rec.Body.String())
	}
	_ = rec.Result().Body.Close()
	var prop configintents.Proposal
	if err := json.NewDecoder(strings.NewReader(rec.Body.String())).Decode(&prop); err != nil {
		t.Fatalf("decode proposal: %v", err)
	}
	if prop.Op != configintents.OpListAdd {
		t.Errorf("op = %q, want %q", prop.Op, configintents.OpListAdd)
	}
	if prop.Path != configintents.PathSSRFIPAllowlist {
		t.Errorf("path = %q, want %q", prop.Path, configintents.PathSSRFIPAllowlist)
	}
	if prop.Value != "10.1.2.3/32" {
		t.Errorf("value = %q, want %q", prop.Value, "10.1.2.3/32")
	}

	// Unknown reason → 422.
	req2 := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/config/unblock-proposal", strings.NewReader(`{"target":"1.2.3.4","reason":"nope"}`))
	req2.AddCookie(cookie)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	_ = rec2.Result().Body.Close()
	if rec2.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown reason: got %d, want 422", rec2.Code)
	}
}
