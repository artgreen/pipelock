// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Command gen reads internal/config/schema.go via go/ast and emits a
// machine-readable JSON descriptor of the whole config schema. It is run from
// the parent configschema directory via `go generate` (see descriptor.go):
//
//	cd internal/config/configschema && go run ./gen
//
// so the schema source is at ../schema.go and the output is ./descriptor.json.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"reflect"
	"strings"

	"github.com/luckyPipewrench/pipelock/internal/config"
	"github.com/luckyPipewrench/pipelock/internal/config/configschema"
)

const (
	// schemaPath is the schema source relative to the run cwd (configschema dir).
	schemaPath = "../schema.go"
	// outputPath is the descriptor written relative to the run cwd.
	outputPath = "descriptor.json"
	// rootStruct is the type the walk starts from.
	rootStruct = "Config"
	// maxDepth bounds recursion so a (hypothetical) cyclic struct graph cannot
	// loop forever.
	maxDepth = 12
)

// enums maps a yaml key to the closed set of values a string field with that
// key accepts. The field walker upgrades a plain-string field to TypeEnum when
// its key appears here. Keyed by yaml key, not by go type, because the same
// enum applies across many structs (every "action" field, etc.).
var enums = map[string][]string{
	"mode":         {config.ModeStrict, config.ModeBalanced, config.ModeAudit, config.ModePermissive},
	"action":       {config.ActionBlock, config.ActionRedirect, config.ActionWarn, config.ActionAsk, config.ActionStrip, config.ActionForward, config.ActionAllow},
	"min_action":   {config.ActionBlock, config.ActionRedirect, config.ActionWarn, config.ActionAsk, config.ActionStrip, config.ActionForward, config.ActionAllow},
	"severity":     {config.SeverityInfo, config.SeverityWarn, config.SeverityCritical, config.SeverityHigh, config.SeverityMedium},
	"min_severity": {config.SeverityInfo, config.SeverityWarn, config.SeverityCritical, config.SeverityHigh, config.SeverityMedium},
	"header_mode":  {config.HeaderModeSensitive, config.HeaderModeAll},
}

// advancedTypes are bare go type names whose custom YAML unmarshalers mean the
// generator cannot offer a safe structured editor; fields of these types are
// marked AdvancedOnly.
var advancedTypes = map[string]bool{
	"WatchPath":            true,
	"LearnLockEnvironment": true,
}

// fieldInfo is one exported, yaml-tagged struct field captured from the AST.
type fieldInfo struct {
	name    string // go field name
	yamlKey string // yaml tag key (before any comma options)
	goType  string // rendered go type string
	doc     string // trimmed doc/line comment
}

// classify maps a rendered go type string to a FieldType. The enum upgrade is
// applied by the field walker (by yaml key), not here: classify returns
// TypeString for plain strings.
func classify(goType string, structNames map[string]bool) configschema.FieldType {
	switch goType {
	case "*bool":
		return configschema.TypeTriState
	case "bool":
		return configschema.TypeBool
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr":
		return configschema.TypeInt
	case "float32", "float64":
		return configschema.TypeFloat
	case "string":
		return configschema.TypeString
	case "[]string":
		return configschema.TypeList
	case "map[string]string":
		return configschema.TypeMap
	}
	// A qualified type (pkg.Name, []pkg.Name, ...) is never a locally-declared
	// struct, so it must never be treated as a group — even when its selector
	// name collides with a local struct (e.g. redact.Config vs the root Config).
	// This check must precede the structNames lookup.
	if strings.Contains(goType, ".") {
		return configschema.TypeOpaque
	}
	if structNames[goType] {
		return configschema.TypeGroup
	}
	// []SomeStruct, map[string]SomeStruct, custom/unmarshaler types,
	// cross-package types, etc.
	return configschema.TypeOpaque
}

// isSecretKey reports whether a yaml key names a secret-bearing field whose
// value must never be surfaced to the console UI in plaintext.
func isSecretKey(key string) bool {
	switch key {
	case "api_token", "dsn", "session_secret", "auth_token":
		return true
	}
	return strings.Contains(key, "secret") ||
		strings.Contains(key, "password") ||
		strings.Contains(key, "private_key")
}

// label renders a yaml key as a human-readable title (fetch_proxy -> Fetch Proxy).
func label(key string) string {
	parts := strings.Split(key, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// typeString renders an ast.Expr type node to a go type string the classifier
// understands. Cross-package selector types keep their package qualifier
// (e.g. redact.Config, []redact.Rule) so they can never collide with a
// locally-declared struct name; classify treats any qualified type as opaque.
func typeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + typeString(t.X)
	case *ast.ArrayType:
		return "[]" + typeString(t.Elt)
	case *ast.MapType:
		return "map[" + typeString(t.Key) + "]" + typeString(t.Value)
	case *ast.SelectorExpr:
		// Cross-package type (pkg.Name); keep the package qualifier. The "."
		// in the result is what marks it qualified (and thus opaque).
		return typeString(t.X) + "." + t.Sel.Name
	default:
		return ""
	}
}

// docText extracts trimmed help text from a field's doc and line comments,
// preferring the doc comment (above the field) and falling back to the inline
// trailing comment.
func docText(f *ast.Field) string {
	var src *ast.CommentGroup
	switch {
	case f.Doc != nil:
		src = f.Doc
	case f.Comment != nil:
		src = f.Comment
	default:
		return ""
	}
	var lines []string
	for _, c := range src.List {
		line := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		line = strings.TrimSpace(strings.TrimPrefix(line, "/*"))
		line = strings.TrimSpace(strings.TrimSuffix(line, "*/"))
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, " "))
}

// yamlKey extracts the yaml tag key from a struct tag literal. Returns "" when
// there is no yaml tag or the tag is yaml:"-".
func yamlKey(tag string) string {
	if tag == "" {
		return ""
	}
	// tag is the raw literal including backticks; reflect.StructTag wants the
	// content without the surrounding quotes.
	tag = strings.Trim(tag, "`")
	val, ok := reflect.StructTag(tag).Lookup("yaml")
	if !ok {
		return ""
	}
	key := val
	if i := strings.IndexByte(key, ','); i >= 0 {
		key = key[:i]
	}
	if key == "" || key == "-" {
		return ""
	}
	return key
}

// collectStructs parses the schema file and returns the set of locally-declared
// struct type names plus a map from struct name to its exported, yaml-tagged
// fields in declaration order.
func collectStructs(file *ast.File) (map[string]bool, map[string][]fieldInfo) {
	names := map[string]bool{}
	fields := map[string][]fieldInfo{}

	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			names[ts.Name.Name] = true
			fields[ts.Name.Name] = structFields(st)
		}
	}
	return names, fields
}

// structFields extracts the exported, yaml-tagged fields of a struct.
func structFields(st *ast.StructType) []fieldInfo {
	var out []fieldInfo
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			continue // embedded field; schema has none we need to descend.
		}
		var tag string
		if f.Tag != nil {
			tag = f.Tag.Value
		}
		key := yamlKey(tag)
		if key == "" {
			continue // no yaml tag, or yaml:"-".
		}
		goType := typeString(f.Type)
		if goType == "" {
			continue // unrenderable type (func, chan, etc.); not in schema.
		}
		for _, name := range f.Names {
			if !name.IsExported() {
				continue
			}
			out = append(out, fieldInfo{
				name:    name.Name,
				yamlKey: key,
				goType:  goType,
				doc:     docText(f),
			})
		}
	}
	return out
}

// tristateDefault parses a "nil = <true|false>" hint out of a tri-state field's
// doc comment. Returns the bool and true when a hint is found.
func tristateDefault(doc string) (bool, bool) {
	lower := strings.ToLower(doc)
	idx := strings.Index(lower, "nil =")
	if idx < 0 {
		idx = strings.Index(lower, "nil/")
		if idx >= 0 {
			rest := lower[idx:]
			if strings.HasPrefix(rest, "nil/true") {
				return true, true
			}
			if strings.HasPrefix(rest, "nil/false") {
				return false, true
			}
		}
		return false, false
	}
	rest := strings.TrimSpace(lower[idx+len("nil ="):])
	switch {
	case strings.HasPrefix(rest, "true"):
		return true, true
	case strings.HasPrefix(rest, "false"):
		return false, true
	}
	return false, false
}

// builder walks the struct graph from Config and produces the field tree.
type builder struct {
	structFields map[string][]fieldInfo
	structNames  map[string]bool
	defaults     *config.Config
	leaves       int
}

// buildFields turns the fields of struct typeName into Field nodes, recursing
// into group (nested struct) fields. parentPath is the dotted path of the
// enclosing struct ("" at the root).
func (b *builder) buildFields(typeName, parentPath string, depth int) []configschema.Field {
	if depth > maxDepth {
		return nil
	}
	var out []configschema.Field
	for _, fi := range b.structFields[typeName] {
		ft := classify(fi.goType, b.structNames)

		// Enum upgrade: a plain-string field whose yaml key names an enum.
		if ft == configschema.TypeString {
			if _, ok := enums[fi.yamlKey]; ok {
				ft = configschema.TypeEnum
			}
		}

		path := fi.yamlKey
		if parentPath != "" {
			path = parentPath + "." + fi.yamlKey
		}

		field := configschema.Field{
			Path:         path,
			Key:          fi.yamlKey,
			Label:        label(fi.yamlKey),
			Type:         ft,
			Help:         fi.doc,
			Secret:       isSecretKey(fi.yamlKey),
			AdvancedOnly: ft == configschema.TypeOpaque || advancedTypes[bareType(fi.goType)],
		}
		if ft == configschema.TypeEnum {
			field.Enum = enums[fi.yamlKey]
		}

		switch ft {
		case configschema.TypeGroup:
			field.Children = b.buildFields(fi.goType, path, depth+1)
		case configschema.TypeOpaque:
			b.leaves++
		case configschema.TypeTriState:
			b.leaves++
			// A tri-state's struct default is nil; the effective default lives
			// in the doc comment ("nil = true").
			if v, ok := tristateDefault(fi.doc); ok {
				field.Default = v
			} else if rv, ok := b.reflectValue(path); ok {
				// Fall back to a non-nil pointer default if Defaults() set one.
				if pv := reflect.ValueOf(rv); pv.Kind() == reflect.Pointer && !pv.IsNil() {
					field.Default = pv.Elem().Bool()
				}
			}
		default:
			b.leaves++
			field.Default = b.defaultFor(path, fi.goType)
		}

		out = append(out, field)
	}
	return out
}

// bareType strips pointer/slice/map decoration to the underlying type name so
// AdvancedOnly detection catches []WatchPath as well as WatchPath.
func bareType(goType string) string {
	t := strings.TrimPrefix(goType, "*")
	t = strings.TrimPrefix(t, "[]")
	if i := strings.LastIndexByte(t, ']'); i >= 0 {
		t = t[i+1:] // map[...]Value -> Value
	}
	return t
}

// defaultFor reflects the default value of a leaf field out of config.Defaults()
// by walking struct fields by yaml tag. Tri-state (*bool) defaults are resolved
// in buildFields from the doc comment, not here. Returns nil when no meaningful
// default exists.
func (b *builder) defaultFor(path, _ string) any {
	if v, ok := b.reflectValue(path); ok {
		return v
	}
	return nil
}

// reflectValue walks config.Defaults() by yaml key path and returns the leaf
// value (and true) when found. Unexported fields are skipped via PkgPath.
func (b *builder) reflectValue(path string) (any, bool) {
	parts := strings.Split(path, ".")
	cur := reflect.ValueOf(b.defaults).Elem()
	for _, key := range parts {
		if cur.Kind() == reflect.Pointer {
			if cur.IsNil() {
				return nil, false
			}
			cur = cur.Elem()
		}
		if cur.Kind() != reflect.Struct {
			return nil, false
		}
		next, ok := fieldByYAMLKey(cur, key)
		if !ok {
			return nil, false
		}
		cur = next
	}
	if !cur.IsValid() || !cur.CanInterface() {
		return nil, false
	}
	return cur.Interface(), true
}

// fieldByYAMLKey returns the struct field of v whose yaml tag key matches key.
// Unexported fields are skipped (PkgPath != "").
func fieldByYAMLKey(v reflect.Value, key string) (reflect.Value, bool) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" {
			continue // unexported.
		}
		tag := sf.Tag.Get("yaml")
		k := tag
		if j := strings.IndexByte(k, ','); j >= 0 {
			k = k[:j]
		}
		if k == key {
			return v.Field(i), true
		}
	}
	return reflect.Value{}, false
}

func run() error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, schemaPath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", schemaPath, err)
	}

	structNames, fields := collectStructs(file)
	if _, ok := fields[rootStruct]; !ok {
		return fmt.Errorf("root struct %q not found in %s", rootStruct, schemaPath)
	}

	b := &builder{
		structFields: fields,
		structNames:  structNames,
		defaults:     config.Defaults(),
	}

	sections := b.buildFields(rootStruct, "", 0)
	desc := configschema.Descriptor{
		FieldCount: b.leaves,
		Sections:   sections,
	}

	out, err := json.MarshalIndent(desc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling descriptor: %w", err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(outputPath, out, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", outputPath, err)
	}

	log.Printf("configschema: wrote %s with %d leaf fields", outputPath, b.leaves)
	return nil
}

func main() {
	log.SetFlags(0)
	if err := run(); err != nil {
		log.Fatalf("configschema gen: %v", err)
	}
}
