// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package configsvc reads, validates, and atomically writes the pipelock.yaml
// managed by the console. Validation reuses pipelock's real config package so
// it can never drift from the running proxy.
package configsvc

import (
	"bytes"
	"errors"
	"io"

	pcfg "github.com/luckyPipewrench/pipelock/internal/config"
	"gopkg.in/yaml.v3"
)

// ValidationResult reports whether submitted YAML is a valid pipelock config.
type ValidationResult struct {
	OK       bool     `json:"ok"`
	Error    string   `json:"error,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// Validate parses raw YAML with the same strict decoder pipelock uses
// (unknown fields rejected), applies config defaults so that sparse configs
// are treated the same way pipelock treats them at startup, and then runs
// the real validator.
func Validate(raw []byte) ValidationResult {
	cfg := &pcfg.Config{}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
		return ValidationResult{OK: false, Error: err.Error()}
	}
	cfg.ApplyDefaults()
	warns, err := cfg.ValidateWithWarnings()
	res := ValidationResult{OK: err == nil}
	if err != nil {
		res.Error = err.Error()
	}
	for _, w := range warns {
		res.Warnings = append(res.Warnings, w.Field+": "+w.Message)
	}
	return res
}
