// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package configschema is a generated, machine-readable description of the
// pipelock config schema (every field's path, type, default, help, and flags).
// It drives the console's structured settings UI. Regenerate with `go generate`.
package configschema

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:generate go run ./gen

//go:embed descriptor.json
var descriptorJSON []byte

// FieldType enumerates how a field is rendered/edited.
type FieldType string

const (
	TypeGroup    FieldType = "group"
	TypeBool     FieldType = "bool"
	TypeTriState FieldType = "tristate"
	TypeInt      FieldType = "int"
	TypeFloat    FieldType = "float"
	TypeString   FieldType = "string"
	TypeEnum     FieldType = "enum"
	TypeList     FieldType = "list"
	TypeMap      FieldType = "map"
	TypeOpaque   FieldType = "opaque"
)

// Field is one node of the schema tree.
type Field struct {
	Path         string    `json:"path"`
	Key          string    `json:"key"`
	Label        string    `json:"label"`
	Type         FieldType `json:"type"`
	Help         string    `json:"help,omitempty"`
	Default      any       `json:"default,omitempty"`
	Enum         []string  `json:"enum,omitempty"`
	Secret       bool      `json:"secret,omitempty"`
	AdvancedOnly bool      `json:"advanced_only,omitempty"`
	Children     []Field   `json:"children,omitempty"`
}

// Descriptor is the whole schema tree (top-level sections).
type Descriptor struct {
	FieldCount int     `json:"field_count"`
	Sections   []Field `json:"sections"`
}

// Load parses the embedded descriptor.
func Load() (*Descriptor, error) {
	var d Descriptor
	if err := json.Unmarshal(descriptorJSON, &d); err != nil {
		return nil, fmt.Errorf("parsing embedded descriptor: %w", err)
	}
	return &d, nil
}

// Help returns the help text for a dotted path, or "" if unknown.
func (d *Descriptor) Help(path string) string {
	var walk func(fs []Field) string
	walk = func(fs []Field) string {
		for i := range fs {
			if fs[i].Path == path {
				return fs[i].Help
			}
			if len(fs[i].Children) > 0 {
				if h := walk(fs[i].Children); h != "" {
					return h
				}
			}
		}
		return ""
	}
	return walk(d.Sections)
}
