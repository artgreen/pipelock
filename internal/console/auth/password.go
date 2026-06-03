// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package auth provides single-password authentication for the console.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16

	// Upper bounds on argon2 parameters parsed from a stored (untrusted) hash.
	// argon2.IDKey panics when time<1 or threads<1, and an unbounded memory
	// param requests terabytes of RAM. A tampered hash must fail closed, never
	// crash, so verify-time params are clamped to these ceilings.
	maxArgonMemory  = 256 * 1024 // 256 MiB ceiling to bound verify cost
	maxArgonTime    = 16
	maxArgonThreads = 16
)

// HashPassword returns an encoded argon2id hash of the password.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether password matches the encoded argon2id hash.
func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	// Treat parsed params as untrusted: reject out-of-range values so the
	// argon2.IDKey call below cannot panic (t<1 / p<1) or OOM (huge m).
	if m < 1 || m > maxArgonMemory || t < 1 || t > maxArgonTime || p < 1 || p > maxArgonThreads {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	keyLen := len(want)
	if keyLen < 1 || keyLen > 1<<20 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(keyLen)) //nolint:gosec // bounds-checked above
	return subtle.ConstantTimeCompare(got, want) == 1
}
