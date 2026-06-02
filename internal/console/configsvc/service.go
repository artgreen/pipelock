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
