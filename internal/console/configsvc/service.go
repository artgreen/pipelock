// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package configsvc reads, validates, and atomically writes the pipelock.yaml
// managed by the console. Validation reuses pipelock's real config package so
// it can never drift from the running proxy.
package configsvc

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	pcfg "github.com/luckyPipewrench/pipelock/internal/config"
)

// Service manages the on-disk pipelock config at Path.
type Service struct {
	Path string
}

// New returns a Service for the pipelock config at path.
func New(path string) *Service { return &Service{Path: path} }

// Read returns the current pipelock.yaml contents.
func (s *Service) Read() ([]byte, error) {
	data, err := os.ReadFile(filepath.Clean(s.Path))
	if err != nil {
		return nil, fmt.Errorf("reading pipelock config: %w", err)
	}
	return data, nil
}

// ValidationResult reports whether submitted YAML is a valid pipelock config.
type ValidationResult struct {
	OK       bool     `json:"ok"`
	Error    string   `json:"error,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// nowFunc is overridable in tests for deterministic backup names.
var nowFunc = time.Now

// Write validates raw config and, only if valid, backs up the current file and
// atomically replaces it. Invalid input is rejected and nothing is written.
func (s *Service) Write(raw []byte) error {
	if res := Validate(raw); !res.OK {
		return fmt.Errorf("config rejected: %s", res.Error)
	}
	current, err := os.ReadFile(filepath.Clean(s.Path))
	if err != nil {
		return fmt.Errorf("reading current config for backup: %w", err)
	}
	backup := fmt.Sprintf("%s.bak.%s", s.Path, nowFunc().UTC().Format("20060102T150405Z"))
	if err := os.WriteFile(backup, current, 0o600); err != nil {
		return fmt.Errorf("writing backup: %w", err)
	}

	dir := filepath.Dir(s.Path)
	tmp, err := os.CreateTemp(dir, ".pipelock-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp config: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("setting temp config permissions: %w", err)
	}
	if _, err := tmp.Write(raw); err != nil {
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
	return os.Rename(tmpName, s.Path)
}

// Validate validates raw YAML with full Load fidelity by delegating to
// config.ValidateBytes: strict decode (unknown fields rejected),
// single-document enforcement, security-boolean fail-closed defaults, normal
// defaults, then full validation. Delegating keeps the config package as the
// single source of truth so console validation cannot drift from startup.
func Validate(raw []byte) ValidationResult {
	warns, err := pcfg.ValidateBytes(raw)
	res := ValidationResult{OK: err == nil}
	if err != nil {
		res.Error = err.Error()
	}
	for _, w := range warns {
		res.Warnings = append(res.Warnings, w.Field+": "+w.Message)
	}
	return res
}
