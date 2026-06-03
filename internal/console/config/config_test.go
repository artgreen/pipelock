// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaultsAndOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pipelock-console.yaml")
	yaml := "listen: \"127.0.0.1:9999\"\npipelock:\n  base_url: \"http://127.0.0.1:8888\"\nconfig_path: \"/tmp/pipelock.yaml\"\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen != "127.0.0.1:9999" {
		t.Errorf("Listen = %q", cfg.Listen)
	}
	if cfg.ServiceUnit != "pipelock" {
		t.Errorf("ServiceUnit default = %q, want pipelock", cfg.ServiceUnit)
	}
	if cfg.Pipelock.KillswitchURL != "http://127.0.0.1:8888" {
		t.Errorf("KillswitchURL should default to base_url, got %q", cfg.Pipelock.KillswitchURL)
	}
}

func TestLoadGeneratesSessionSecret(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(path, []byte("config_path: /tmp/p.yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.SessionSecret) != 64 {
		t.Errorf("expected generated session_secret, got %q", cfg.SessionSecret)
	}
	cfg2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.SessionSecret != cfg.SessionSecret {
		t.Errorf("session_secret not persisted across loads")
	}
}

func TestSaveRoundTripsAdminPasswordHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(path, []byte("config_path: /tmp/p.yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.AdminPasswordHash = "$argon2id$v=19$..."
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.AdminPasswordHash != "$argon2id$v=19$..." {
		t.Errorf("hash not persisted: %q", reloaded.AdminPasswordHash)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("config file mode = %o, want 0o600", info.Mode().Perm())
	}
}

// TestEffectiveAPITokenPrefersEnvOverride verifies the env override wins at
// runtime but never lands on the config struct, so it cannot be persisted by a
// later Save (e.g. the first-run admin-password write).
func TestEffectiveAPITokenPrefersEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	const existing = "config_path: /tmp/p.yaml\nsession_secret: deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef\npipelock:\n  api_token: disk-token\n"
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	const envToken = "env-secret-token-xyz"
	t.Setenv("PIPELOCK_KILLSWITCH_API_TOKEN", envToken)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Pipelock.APIToken != "disk-token" {
		t.Errorf("on-disk APIToken on struct = %q, want unchanged disk value", cfg.Pipelock.APIToken)
	}
	if got := cfg.EffectiveAPIToken(); got != envToken {
		t.Errorf("EffectiveAPIToken = %q, want env override %q", got, envToken)
	}
}

// TestEnvTokenNeverPersistedAcrossSave is the regression guard for the env
// token leaking to disk: after Load applies no override to the struct, a Save
// (such as persisting the first-run admin password) must not write the env
// token. Fails if the override is ever baked back onto cfg before a Save.
func TestEnvTokenNeverPersistedAcrossSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	// No session_secret: first-run Save fires inside Load.
	if err := os.WriteFile(path, []byte("config_path: /tmp/p.yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	const envToken = "env-secret-token-firstrun"
	t.Setenv("PIPELOCK_KILLSWITCH_API_TOKEN", envToken)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.EffectiveAPIToken(); got != envToken {
		t.Errorf("EffectiveAPIToken = %q, want env override %q", got, envToken)
	}
	// Simulate the first-run admin-password persistence path.
	cfg.AdminPasswordHash = "$argon2id$v=19$..."
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), envToken) {
		t.Errorf("env token must never be persisted to disk, found in: %s", raw)
	}
}

func TestLoadDefaultsListenToLoopback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(path, []byte("config_path: /tmp/p.yaml\nsession_secret: deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != "127.0.0.1:9443" {
		t.Errorf("default Listen = %q, want loopback 127.0.0.1:9443", cfg.Listen)
	}
}

func TestLoadRejectsBadYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(path, []byte(":\n  - [unbalanced"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("expected error loading invalid YAML, got nil")
	}
}
