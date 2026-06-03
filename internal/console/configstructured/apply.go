// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package configstructured

import (
	"bytes"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// ApplyChanges applies a sparse path->value patch onto the raw YAML and returns
// the re-serialized bytes (2-space indent, pipelock convention). A nil value
// deletes the key (revert to default). A value equal to RedactedSentinel is
// skipped (leave the existing secret untouched). help(path) supplies the head
// comment for newly inserted fields.
func ApplyChanges(raw []byte, changes map[string]any, help func(path string) string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	paths := make([]string, 0, len(changes))
	for p := range changes {
		paths = append(paths, p)
	}
	sort.Strings(paths) // deterministic application order
	for _, p := range paths {
		v := changes[p]
		if s, ok := v.(string); ok && s == RedactedSentinel {
			continue // unchanged secret
		}
		val := v
		if v == nil {
			val = DeleteSentinel
		}
		if err := ApplyChange(&doc, p, val, help(p)); err != nil {
			return nil, fmt.Errorf("applying %q: %w", p, err)
		}
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, fmt.Errorf("serializing config: %w", err)
	}
	_ = enc.Close()
	return buf.Bytes(), nil
}
