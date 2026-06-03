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

func TestVerifyRejectsHostileParams(t *testing.T) {
	valid, _ := HashPassword("pw")
	// craft a valid-base64 salt+key but hostile params; reuse the real hash's segments
	hostile := []string{
		"",
		"not-a-hash",
		"$argon2id$",
		"$argon2id$v=19$m=65536,t=0,p=4$AAAA$AAAA",      // t=0 → would panic
		"$argon2id$v=19$m=65536,t=1,p=0$AAAA$AAAA",      // p=0 → would panic
		"$argon2id$v=19$m=4294967295,t=1,p=4$AAAA$AAAA", // huge m → would OOM
		"$argon2id$v=20$m=65536,t=1,p=4$AAAA$AAAA",      // wrong version
		"$argon2id$v=19$m=x$$",                          // junk params
	}
	for _, h := range hostile {
		if VerifyPassword(h, "pw") {
			t.Errorf("hostile hash must not verify: %q", h)
		}
	}
	_ = valid // sanity: the real hash still verifies
	if !VerifyPassword(valid, "pw") {
		t.Error("legit hash must still verify after guards")
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
