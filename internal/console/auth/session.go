// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sync"
)

const cookieName = "pipelock_console_session"

// Options configures a Manager.
type Options struct {
	PasswordHash string
	SecretHex    string
}

// Manager handles login, session cookies, and auth middleware.
type Manager struct {
	passwordHash string
	secret       []byte
	mu           sync.Mutex
	sessions     map[string]struct{}
}

// NewManager constructs a Manager.
func NewManager(o Options) *Manager {
	secret, _ := hex.DecodeString(o.SecretHex)
	return &Manager{passwordHash: o.PasswordHash, secret: secret, sessions: make(map[string]struct{})}
}

// NeedsSetup reports whether no admin password has been set yet.
func (m *Manager) NeedsSetup() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.passwordHash == ""
}

// SetPasswordHash updates the active password hash (used by first-run wizard).
func (m *Manager) SetPasswordHash(hash string) {
	m.mu.Lock()
	m.passwordHash = hash
	m.mu.Unlock()
}

func (m *Manager) sign(token string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

// Login verifies the password and, on success, sets a session cookie.
func (m *Manager) Login(w http.ResponseWriter, password string) bool {
	m.mu.Lock()
	hash := m.passwordHash
	m.mu.Unlock()
	if hash == "" || !VerifyPassword(hash, password) {
		return false
	}
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	token := hex.EncodeToString(raw)
	m.mu.Lock()
	m.sessions[token] = struct{}{}
	m.mu.Unlock()
	// Secure flag is omitted here; the server layer adds it when TLS is active.
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure flag added at TLS termination layer
		Name:     cookieName,
		Value:    token + "." + m.sign(token),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	return true
}

// Logout invalidates the request's session.
func (m *Manager) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		if token, _, ok := splitToken(c.Value); ok {
			m.mu.Lock()
			delete(m.sessions, token)
			m.mu.Unlock()
		}
	}
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure flag added at TLS termination layer; this cookie clears the session
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func splitToken(v string) (token, sig string, ok bool) {
	for i := 0; i < len(v); i++ {
		if v[i] == '.' {
			return v[:i], v[i+1:], true
		}
	}
	return "", "", false
}

func (m *Manager) valid(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	token, sig, ok := splitToken(c.Value)
	if !ok || !hmac.Equal([]byte(sig), []byte(m.sign(token))) {
		return false
	}
	m.mu.Lock()
	_, exists := m.sessions[token]
	m.mu.Unlock()
	return exists
}

// RequireAuth wraps a handler, returning 401 for unauthenticated requests.
func (m *Manager) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.valid(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
