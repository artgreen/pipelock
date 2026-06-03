// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// ValidateBytes validates a raw pipelock config exactly as Load would, minus
// file IO and license resolution: strict decode (unknown fields rejected),
// single-document enforcement, security-boolean fail-closed defaults, normal
// defaults, then full validation. It lets external callers (e.g. the console)
// validate submitted config without drifting from startup behavior.
//
// Note: file-location-dependent checks that Load performs (license resolution,
// relative path resolution, and file_sentry watch-path containment) are
// intentionally NOT run here — they require the real config file path. A config
// that passes ValidateBytes may still be rejected by pipelock at startup for
// those reasons (the safe direction: pipelock remains the final gate).
func ValidateBytes(raw []byte) ([]Warning, error) {
	cfg := &Config{}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err == nil {
		return nil, fmt.Errorf("parsing config: multiple YAML documents not supported (pipelock config must be a single document)")
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	cfg.rawBytes = raw
	applySecurityDefaults(raw, cfg)
	cfg.ApplyDefaults()
	return cfg.ValidateWithWarnings()
}
