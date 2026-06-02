// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
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
	if len(cfg.SessionSecret) < 32 {
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
}
