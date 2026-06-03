// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package configstructured

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/luckyPipewrench/pipelock/internal/config"
)

// RedactedSentinel replaces secret field values in EffectiveValues output so a
// token is never sent to the browser. A patch that submits this value back is
// treated by the caller as "leave the existing secret untouched".
const RedactedSentinel = "__redacted__"

// EffectiveValues returns the config as a nested map keyed by yaml tags, with
// pipelock defaults applied (so the UI shows effective values), and the given
// secretPaths redacted to RedactedSentinel.
func EffectiveValues(raw []byte, secretPaths []string) (map[string]any, error) {
	cfg := &config.Config{}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	cfg.ApplyDefaults()
	// Round-trip through yaml to get a nested map keyed by yaml tags.
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshaling config: %w", err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(out, &m); err != nil {
		return nil, fmt.Errorf("re-parsing config: %w", err)
	}
	for _, p := range secretPaths {
		redactPath(m, strings.Split(p, "."))
	}
	return m, nil
}

// redactPath replaces the leaf at the dotted path with RedactedSentinel if the
// leaf exists and is non-empty (a non-empty string secret). Absent or empty
// leaves are left as-is (nothing to hide).
func redactPath(m map[string]any, segs []string) {
	if len(segs) == 0 {
		return
	}
	if len(segs) == 1 {
		if v, ok := m[segs[0]]; ok {
			if s, isStr := v.(string); !isStr || s != "" {
				m[segs[0]] = RedactedSentinel
			}
		}
		return
	}
	if child, ok := m[segs[0]].(map[string]any); ok {
		redactPath(child, segs[1:])
	}
}

// PresentPaths returns the set of dotted paths explicitly present in raw
// (leaves and intermediate maps), so the UI can distinguish set vs default.
func PresentPaths(raw []byte) map[string]bool {
	var m map[string]any
	if err := yaml.Unmarshal(raw, &m); err != nil || m == nil {
		return map[string]bool{}
	}
	out := map[string]bool{}
	var walk func(node map[string]any, prefix string)
	walk = func(node map[string]any, prefix string) {
		for k, v := range node {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			out[path] = true
			if child, ok := v.(map[string]any); ok {
				walk(child, path)
			}
		}
	}
	walk(m, "")
	return out
}
