// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		goType string
		want   string
	}{
		{"*bool", "tristate"},
		{"bool", "bool"},
		{"int", "int"},
		{"float64", "float"},
		{"string", "string"},
		{"[]string", "list"},
		{"map[string]string", "map"},
		{"FetchProxy", "group"},
		{"[]SuppressEntry", "opaque"},
		{"redact.Config", "opaque"},
		{"[]redact.Rule", "opaque"},
	}
	// "Config" is intentionally present to prove a qualified selector type
	// (redact.Config) is opaque even when its bare name collides with a local struct.
	structNames := map[string]bool{"FetchProxy": true, "SuppressEntry": true, "Monitoring": true, "Config": true}
	for _, c := range cases {
		if got := classify(c.goType, structNames); string(got) != c.want {
			t.Errorf("classify(%q) = %q, want %q", c.goType, got, c.want)
		}
	}
}

func TestSecretAndLabel(t *testing.T) {
	if !isSecretKey("api_token", "string") || !isSecretKey("dsn", "string") {
		t.Error("api_token/dsn should be secret")
	}
	if isSecretKey("mode", "string") {
		t.Error("mode is not secret")
	}
	// A file PATH containing "secret" must not be flagged: it is not a secret value.
	if isSecretKey("secrets_file", "string") {
		t.Error("secrets_file is a path, not a secret")
	}
	// An int field containing "secret" in its key must not be flagged.
	if isSecretKey("min_env_secret_length", "int") {
		t.Error("min_env_secret_length is an int length, not a secret")
	}
	if label("fetch_proxy") != "Fetch Proxy" {
		t.Errorf("label = %q", label("fetch_proxy"))
	}
}
