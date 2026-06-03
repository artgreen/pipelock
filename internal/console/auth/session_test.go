// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestManager(t *testing.T, hash string) *Manager {
	t.Helper()
	return NewManager(Options{PasswordHash: hash, SecretHex: "00112233445566778899aabbccddeeff"})
}

func TestMiddlewareBlocksUnauthenticated(t *testing.T) {
	m := newTestManager(t, "$argon2id$dummy")
	protected := m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLoginThenAccess(t *testing.T) {
	hash, _ := HashPassword("pw")
	m := newTestManager(t, hash)

	loginRec := httptest.NewRecorder()
	if !m.Login(loginRec, "pw") {
		t.Fatal("login with correct password failed")
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie issued")
	}

	protected := m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.AddCookie(cookies[0])
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("authenticated request status = %d", rec.Code)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	hash, _ := HashPassword("right")
	m := newTestManager(t, hash)
	rec := httptest.NewRecorder()
	if m.Login(rec, "wrong") {
		t.Error("login should fail with wrong password")
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Error("no cookie should be set on failed login")
	}
}

func TestLogoutInvalidatesSession(t *testing.T) {
	hash, _ := HashPassword("pw")
	m := newTestManager(t, hash)
	loginRec := httptest.NewRecorder()
	m.Login(loginRec, "pw")
	cookie := loginRec.Result().Cookies()[0]

	logoutReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/logout", nil)
	logoutReq.AddCookie(cookie)
	m.Logout(httptest.NewRecorder(), logoutReq)

	protected := m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("logged-out session should be rejected, got %d", rec.Code)
	}
}

func TestForgedCookieRejected(t *testing.T) {
	hash, _ := HashPassword("pw")
	m := newTestManager(t, hash)
	protected := m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "pipelock_console_session", Value: "deadbeef.deadbeef"}) //nolint:gosec // intentionally bare forged cookie for rejection test
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("forged cookie must be rejected, got %d", rec.Code)
	}
}

func TestNeedsSetupWhenNoPasswordHash(t *testing.T) {
	m := newTestManager(t, "")
	if !m.NeedsSetup() {
		t.Error("expected NeedsSetup=true with empty hash")
	}
}
