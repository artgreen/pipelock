// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package config loads and persists the pipelock-console application config.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// TLS holds optional cert/key paths. When both are set the server serves HTTPS.
type TLS struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// Pipelock points the console at the pipelock instance it manages.
type Pipelock struct {
	BaseURL       string `yaml:"base_url"`
	KillswitchURL string `yaml:"killswitch_url"`
	APIToken      string `yaml:"api_token"`
}

// ConsoleConfig is the console's own configuration (distinct from pipelock.yaml).
type ConsoleConfig struct {
	Listen            string   `yaml:"listen"`
	TLS               TLS      `yaml:"tls"`
	Pipelock          Pipelock `yaml:"pipelock"`
	ConfigPath        string   `yaml:"config_path"`
	ServiceUnit       string   `yaml:"service_unit"`
	AdminPasswordHash string   `yaml:"admin_password_hash"`
	SessionSecret     string   `yaml:"session_secret"`
}

// Load reads the console config, applies defaults, generates and persists a
// session secret on first use, and returns the resolved config.
func Load(path string) (*ConsoleConfig, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("reading console config %s: %w", path, err)
	}
	cfg := &ConsoleConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing console config %s: %w", path, err)
	}

	if cfg.Listen == "" {
		// Default to loopback. First-run setup (POST /api/setup) is unauthenticated
		// until an admin password exists; binding to all interfaces by default would
		// let any host on the network claim the initial admin and seize config,
		// service-restart, and kill-switch control. Operators opt into external
		// exposure by setting listen explicitly.
		cfg.Listen = "127.0.0.1:9443"
	}
	if cfg.ServiceUnit == "" {
		cfg.ServiceUnit = "pipelock"
	}
	if cfg.Pipelock.BaseURL == "" {
		cfg.Pipelock.BaseURL = "http://127.0.0.1:8888"
	}
	if cfg.Pipelock.KillswitchURL == "" {
		cfg.Pipelock.KillswitchURL = cfg.Pipelock.BaseURL
	}
	if cfg.ConfigPath == "" {
		cfg.ConfigPath = "/usr/local/etc/pipelock.yaml"
	}

	if cfg.SessionSecret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("generating session secret: %w", err)
		}
		cfg.SessionSecret = hex.EncodeToString(b)
		if err := Save(path, cfg); err != nil {
			return nil, fmt.Errorf("persisting session secret: %w", err)
		}
	}

	return cfg, nil
}

// EffectiveAPIToken returns the API token the console should use at runtime,
// preferring the PIPELOCK_KILLSWITCH_API_TOKEN env override over the on-disk
// value. The override is read on demand and intentionally not stored on the
// config struct, so persisting the config (e.g. saving the first-run admin
// password) can never write an env-supplied token to disk.
func (c *ConsoleConfig) EffectiveAPIToken() string {
	if token := os.Getenv("PIPELOCK_KILLSWITCH_API_TOKEN"); token != "" {
		return token
	}
	return c.Pipelock.APIToken
}

// Save writes the console config back to disk atomically (temp + rename).
func Save(path string, cfg *ConsoleConfig) error {
	out, err := yaml.Marshal(cfg) //nolint:gosec // G117: ConsoleConfig is the on-disk config schema; all fields (incl. session_secret, api_token) are intentionally serialized.
	if err != nil {
		return fmt.Errorf("marshaling console config: %w", err)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".pipelock-console-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp config: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("setting temp config permissions: %w", err)
	}
	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temp config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("syncing temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp config: %w", err)
	}
	return os.Rename(tmpName, path)
}
