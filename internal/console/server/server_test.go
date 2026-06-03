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
	"github.com/luckyPipewrench/pipelock/internal/console/configstructured"
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
		{http.MethodGet, "/api/config/schema"},
		{http.MethodGet, "/api/config/values"},
		{http.MethodPost, "/api/config/structured"},
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

func TestConfigSchemaAndValuesEndpoints(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pipelock.yaml")
	seed := "mode: audit\nkill_switch:\n  api_token: \"tok\"\n"
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}
	hash, _ := auth.HashPassword("pw")
	h := New(Deps{
		Auth:    auth.NewManager(auth.Options{PasswordHash: hash, SecretHex: "00112233445566778899aabbccddeeff"}),
		Config:  configsvc.New(path),
		Client:  pipelockclient.New(pipelockclient.Options{BaseURL: "http://127.0.0.1:1"}),
		Service: service.New("pipelock"),
		Buffer:  events.NewBuffer(100),
		Hub:     events.NewHub(),
	})

	// GET /api/config/schema without cookie → 401.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/config/schema", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("schema unauthenticated: got %d, want 401", rec.Code)
	}

	// Login.
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/login", strings.NewReader(`{"password":"pw"}`)))
	cookies := loginRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie after login")
	}
	cookie := cookies[0]

	// GET /api/config/schema (authed) → 200, field_count > 0.
	schemaReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/config/schema", nil)
	schemaReq.AddCookie(cookie)
	schemaRec := httptest.NewRecorder()
	h.ServeHTTP(schemaRec, schemaReq)
	if schemaRec.Code != http.StatusOK {
		t.Fatalf("schema authed: got %d body=%s", schemaRec.Code, schemaRec.Body.String())
	}
	var schemaResp struct {
		FieldCount int `json:"field_count"`
	}
	if err := json.NewDecoder(strings.NewReader(schemaRec.Body.String())).Decode(&schemaResp); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	if schemaResp.FieldCount <= 0 {
		t.Errorf("field_count = %d, want > 0", schemaResp.FieldCount)
	}

	// GET /api/config/values (authed) → 200, effective map with secret redacted, present map with mode=true.
	valReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/config/values", nil)
	valReq.AddCookie(cookie)
	valRec := httptest.NewRecorder()
	h.ServeHTTP(valRec, valReq)
	if valRec.Code != http.StatusOK {
		t.Fatalf("values authed: got %d body=%s", valRec.Code, valRec.Body.String())
	}
	var valResp struct {
		Effective map[string]any  `json:"effective"`
		Present   map[string]bool `json:"present"`
	}
	if err := json.NewDecoder(strings.NewReader(valRec.Body.String())).Decode(&valResp); err != nil {
		t.Fatalf("decode values: %v", err)
	}

	// kill_switch.api_token must be redacted, not "tok".
	ksAny, ok := valResp.Effective["kill_switch"]
	if !ok {
		t.Fatal("effective missing kill_switch key")
	}
	ksMap, ok := ksAny.(map[string]any)
	if !ok {
		t.Fatalf("kill_switch is %T, want map", ksAny)
	}
	if got := ksMap["api_token"]; got != configstructured.RedactedSentinel {
		t.Errorf("api_token = %q, want %q", got, configstructured.RedactedSentinel)
	}

	// present["mode"] must be true.
	if !valResp.Present["mode"] {
		t.Errorf("present[mode] = false, want true")
	}
}

func TestConfigStructuredEndpoint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pipelock.yaml")
	if err := os.WriteFile(path, []byte("mode: audit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hash, _ := auth.HashPassword("pw")
	h := newTestServer(t, path, hash)

	// Unauthenticated request must be 401.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/config/structured", strings.NewReader(`{"changes":{"metrics_listen":"127.0.0.1:9095"}}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: got %d, want 401", rec.Code)
	}

	// Log in.
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/login", strings.NewReader(`{"password":"pw"}`)))
	cookies := loginRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie after login")
	}
	cookie := cookies[0]

	// Valid patch → 204 and file updated.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/config/structured", strings.NewReader(`{"changes":{"metrics_listen":"127.0.0.1:9095"}}`))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("valid patch: got %d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := os.ReadFile(filepath.Clean(path))
	if !strings.Contains(string(got), "metrics_listen: 127.0.0.1:9095") {
		t.Errorf("config not updated: %q", got)
	}

	// Invalid patch (bogus mode value rejected by ValidateBytes) → 400 and file unchanged.
	req2 := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/config/structured", strings.NewReader(`{"changes":{"mode":"bogus-mode"}}`))
	req2.AddCookie(cookie)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("invalid patch: got %d body=%s", rec2.Code, rec2.Body.String())
	}
	got2, _ := os.ReadFile(filepath.Clean(path))
	if !strings.Contains(string(got2), "mode: audit") {
		t.Errorf("file should still contain mode: audit after rejected patch: %q", got2)
	}
}
