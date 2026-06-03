// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package configstructured performs surgical, structure-preserving edits to a
// YAML configuration document. It operates directly on the gopkg.in/yaml.v3
// node tree so that untouched keys, ordering, and comments are left byte-stable
// on the parts of the file it does not modify.
package configstructured

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// deleteMarker is the concrete type behind DeleteSentinel. It is unexported so
// callers cannot construct it any way other than via DeleteSentinel.
type deleteMarker struct{}

// DeleteSentinel is passed as the value to ApplyChange to delete the key at the
// given path. Deleting an absent key is a no-op.
var DeleteSentinel any = deleteMarker{}

// ApplyChange sets, inserts, or deletes the value at a dotted path within a YAML
// document node tree.
//
// doc must be the *yaml.Node produced by yaml.Unmarshal (Kind DocumentNode). An
// empty document is treated as an empty root mapping. path is split on ".";
// each non-final segment must resolve to (or be created as) a mapping. The
// help string, when non-empty and the final key is newly inserted, becomes the
// inserted key's HeadComment.
//
// Existing key nodes (and their attached comments) are preserved on replacement;
// only the value node is swapped. This keeps the edit surgical and never rewrites
// sibling keys or absent sections.
func ApplyChange(doc *yaml.Node, path string, val any, help string) error {
	if doc == nil {
		return fmt.Errorf("nil document node")
	}

	root := rootMapping(doc)

	segments := strings.Split(path, ".")
	if len(segments) == 0 || segments[0] == "" {
		return fmt.Errorf("empty path")
	}

	cur := root
	for _, seg := range segments[:len(segments)-1] {
		next, err := descend(cur, seg)
		if err != nil {
			return err
		}
		cur = next
	}

	last := segments[len(segments)-1]
	return setLeaf(cur, last, val, help)
}

// rootMapping returns the root mapping node of the document, creating an empty
// one when the document is empty.
func rootMapping(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			doc.Content = []*yaml.Node{m}
			return m
		}
		return doc.Content[0]
	}
	// Already a mapping (defensive; ApplyChange documents DocumentNode input).
	return doc
}

// descend finds child key seg in mapping m and returns its value mapping,
// creating an empty mapping when the key is absent. It errors if the key exists
// but its value is not a mapping.
func descend(m *yaml.Node, seg string) (*yaml.Node, error) {
	if m.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("cannot traverse through non-mapping at %q", seg)
	}
	if _, valNode := findKey(m, seg); valNode != nil {
		if valNode.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("cannot traverse through non-mapping at %q", seg)
		}
		return valNode, nil
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: seg}
	valNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	m.Content = append(m.Content, keyNode, valNode)
	return valNode, nil
}

// setLeaf sets, replaces, or deletes key in mapping m.
func setLeaf(m *yaml.Node, key string, val any, help string) error {
	if m.Kind != yaml.MappingNode {
		return fmt.Errorf("cannot set %q: parent is not a mapping", key)
	}

	idx, valNode := findKey(m, key)

	if _, isDelete := val.(deleteMarker); isDelete {
		if idx >= 0 {
			// Remove the key/value pair: Content[idx] is the key, idx+1 the value.
			m.Content = append(m.Content[:idx], m.Content[idx+2:]...)
		}
		return nil
	}

	encoded, err := scalarOrEncoded(val)
	if err != nil {
		return err
	}

	if valNode != nil {
		// Replace value in place, preserving the existing key node + comments.
		m.Content[idx+1] = encoded
		return nil
	}

	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	if help != "" {
		keyNode.HeadComment = help
	}
	m.Content = append(m.Content, keyNode, encoded)
	return nil
}

// findKey returns the index of the key node for key within mapping m and its
// associated value node, or (-1, nil) when absent. Content alternates
// [key, value, key, value, ...].
func findKey(m *yaml.Node, key string) (int, *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return i, m.Content[i+1]
		}
	}
	return -1, nil
}

// scalarOrEncoded builds a yaml.Node for an arbitrary Go value. Node.Encode
// produces a correct scalar/sequence/mapping node for the value, including
// bool/int/float/string and []any/map[string]any. Strings that require quoting
// to round-trip safely are emitted double-quoted.
func scalarOrEncoded(val any) (*yaml.Node, error) {
	n := &yaml.Node{}
	if err := n.Encode(val); err != nil {
		return nil, fmt.Errorf("encode value: %w", err)
	}
	if s, ok := val.(string); ok && needsQuoting(s) {
		n.Style = yaml.DoubleQuotedStyle
	}
	return n, nil
}

// needsQuoting reports whether a string should be force-quoted to round-trip
// safely. yaml.v3's encoder already quotes most ambiguous strings; this only
// adds quoting for leading/trailing whitespace, which the encoder may otherwise
// drop or render ambiguously.
func needsQuoting(s string) bool {
	if s == "" {
		return false
	}
	return strings.TrimSpace(s) != s
}
