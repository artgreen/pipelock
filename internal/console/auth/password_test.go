// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	h, err := HashPassword("hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$") {
		t.Errorf("unexpected hash format: %q", h)
	}
	if !VerifyPassword(h, "hunter2") {
		t.Error("correct password should verify")
	}
	if VerifyPassword(h, "wrong") {
		t.Error("wrong password should not verify")
	}
}

func TestVerifyRejectsMalformedHash(t *testing.T) {
	if VerifyPassword("not-a-hash", "x") {
		t.Error("malformed hash must not verify")
	}
}

func TestHashIsSaltedUnique(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Error("two hashes of the same password must differ (random salt)")
	}
	if !VerifyPassword(h1, "same") || !VerifyPassword(h2, "same") {
		t.Error("both salted hashes must verify the original password")
	}
}
