# Console Schema-Driven Configuration Editor — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A complete, searchable settings UI covering every field in pipelock's config, generated from the Go schema, that saves via surgical YAML-node patches so omitted sections and tri-state security defaults are never altered.

**Architecture:** A build-time AST generator turns `internal/config/schema.go` into a committed JSON descriptor (path/type/default/help/enum/flags for all 566 fields). The console serves the descriptor + current values; edits are applied as a sparse patch over the `yaml.v3` node tree of `pipelock.yaml` (preserving comments and absent sections), then validated and atomically written. The frontend renders a section tree with type-dispatched field widgets, replacing the Phase-1 Guided view.

**Tech Stack:** Go 1.25+ (`go/ast`, `go/parser`, reflect, `gopkg.in/yaml.v3`), React 19 + TypeScript + Vite, Vitest.

Spec: [docs/superpowers/specs/2026-06-03-console-config-schema-editor-design.md](../specs/2026-06-03-console-config-schema-editor-design.md)

**Decisions baked in:** secrets are redacted in `/api/config/values` (never plaintext); enterprise/build-tagged fields are out of scope for v1 (OSS schema only).

---

## File Structure

**Go — descriptor + generator (`internal/config/configschema/`):**
- `descriptor.go` — the `Descriptor`/`Field` types + `Load()` (reads embedded JSON) + `Help(path)`.
- `gen/main.go` — the `go:generate` generator: AST-walk `schema.go`, reflect `Defaults()`, emit `descriptor.json`.
- `descriptor.json` — committed, embedded generated artifact.
- `descriptor_test.go`, `gen/gen_test.go` — golden + coverage tests.

**Go — structured patch + values (`internal/console/configstructured/`):**
- `patch.go` — `ApplyChange(doc *yaml.Node, path string, value any, help string) error` + `Marshal`.
- `values.go` — `EffectiveValues(raw []byte) (map, error)` (defaults-applied, secrets redacted) + `PresentPaths(raw []byte) (map[string]bool)`.
- `patch_test.go`, `values_test.go`.

**Go — endpoints (`internal/console/server/server.go`):** three authed routes + `server_test.go` cases.

**Frontend (`internal/console/web/app/src/`):**
- `api.ts` — `getConfigSchema`, `getConfigValues`, `applyConfigStructured` + types.
- `screens/config/SectionTree.tsx` — left nav + search.
- `screens/config/Field.tsx` — type-dispatching field renderer.
- `screens/config/AllSettings.tsx` — composes tree + fields + save.
- `screens/Config.tsx` — swap Guided → AllSettings.

---

## Phase 1 — Descriptor + generator

### Task 1: Descriptor types + embed

**Files:**
- Create: `internal/config/configschema/descriptor.go`
- Create: `internal/config/configschema/descriptor.json` (temporary stub `{"sections":[]}` to satisfy embed; regenerated in Task 3)
- Test: `internal/config/configschema/descriptor_test.go`

- [ ] **Step 1: Write the failing test**
```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package configschema

import "testing"

func TestLoadDescriptorEmbedded(t *testing.T) {
	d, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d == nil {
		t.Fatal("nil descriptor")
	}
	// Help lookup is total: unknown path returns "" not panic.
	if got := d.Help("nope.nope"); got != "" {
		t.Errorf("Help(unknown) = %q, want empty", got)
	}
}
```

- [ ] **Step 2: Run** `go test ./internal/config/configschema/` → FAIL (package missing).

- [ ] **Step 3: Implement** `descriptor.go`:
```go
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
	TypeGroup    FieldType = "group"    // a nested section (has Children)
	TypeBool     FieldType = "bool"     // plain bool
	TypeTriState FieldType = "tristate" // *bool: nil = default, else explicit
	TypeInt      FieldType = "int"
	TypeFloat    FieldType = "float"
	TypeString   FieldType = "string"
	TypeEnum     FieldType = "enum" // string constrained to Enum values
	TypeList     FieldType = "list" // []string (scalar list)
	TypeMap      FieldType = "map"  // map[string]string
	TypeOpaque   FieldType = "opaque" // complex/unsupported: edit via raw YAML
)

// Field is one node of the schema tree.
type Field struct {
	Path         string    `json:"path"`            // dotted yaml path, e.g. "fetch_proxy.monitoring.blocklist"
	Key          string    `json:"key"`             // leaf yaml key
	Label        string    `json:"label"`           // human label derived from key
	Type         FieldType `json:"type"`
	Help         string    `json:"help,omitempty"`  // Go doc comment
	Default      any       `json:"default,omitempty"`
	Enum         []string  `json:"enum,omitempty"`
	Secret       bool      `json:"secret,omitempty"`
	AdvancedOnly bool      `json:"advanced_only,omitempty"` // edit via raw editor only
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
```
Create `descriptor.json` with stub content: `{"field_count":0,"sections":[]}`.

- [ ] **Step 4: Run** `go test ./internal/config/configschema/` → PASS.

- [ ] **Step 5: Lint + commit**
```bash
gofumpt -w internal/config/configschema/ && golangci-lint run ./internal/config/configschema/
git add internal/config/configschema/
git commit -m "feat(configschema): descriptor types + embedded JSON loader"
```

---

### Task 2: AST generator — structure, types, help, enums, flags

**Files:**
- Create: `internal/config/configschema/gen/main.go`
- Test: `internal/config/configschema/gen/gen_test.go`

The generator parses `schema.go` via `go/ast`, walks the `Config` struct recursively, and classifies each field. It reflects `config.Defaults()` to fill defaults. It writes `descriptor.json`.

- [ ] **Step 1: Write the failing test** (`gen/gen_test.go`) — tests the pure classification helpers the generator exports:
```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		goType string
		want   string
	}{
		{"*bool", "tristate"},
		{"bool", "bool"},
		{"int", "int"},
		{"float64", "float"},
		{"string", "string"},
		{"[]string", "list"},
		{"map[string]string", "map"},
		{"FetchProxy", "group"},      // a known struct type
		{"[]SuppressEntry", "opaque"}, // struct slice → opaque (raw only)
	}
	for _, c := range cases {
		if got := classify(c.goType, structNames); string(got) != c.want {
			t.Errorf("classify(%q) = %q, want %q", c.goType, got, c.want)
		}
	}
}

func TestSecretAndLabel(t *testing.T) {
	if !isSecretKey("api_token") || !isSecretKey("dsn") {
		t.Error("api_token/dsn should be secret")
	}
	if isSecretKey("mode") {
		t.Error("mode is not secret")
	}
	if label("fetch_proxy") != "Fetch Proxy" {
		t.Errorf("label = %q", label("fetch_proxy"))
	}
}
```
(`structNames` is a package var set in tests to e.g. `map[string]bool{"FetchProxy": true, "SuppressEntry": true}`.)

- [ ] **Step 2: Run** `go test ./internal/config/configschema/gen/` → FAIL.

- [ ] **Step 3: Implement** `gen/main.go`. Core pieces (write the full file):
  - Parse `../schema.go` (relative to the gen dir) with `parser.ParseFile` (ParseComments).
  - Collect all `type X struct` names into `structNames`, and each struct's fields (name, go type string via `types`/printer, yaml tag, doc comment, enum hint).
  - `classify(goType, structNames)`: pointer-to-bool → `tristate`; `bool`→`bool`; integer kinds→`int`; `float64`→`float`; `string`→`string` (or `enum` if the field is enum-typed — see below); `[]string`→`list`; `map[string]string`→`map`; a name in `structNames`→`group`; anything else (struct slices, custom types, `[]Struct`, `map[string]Struct`)→`opaque`.
  - Enum detection: a curated map of `yamlKey → enumValues` built from the const blocks — `mode→[strict,balanced,audit,permissive]`, every field whose key is `action`→[block,redirect,warn,ask,strip,forward,allow] (plus `redact` where applicable), `severity`/`min_severity`→[info,warn,critical,high,medium], `header_mode`→[sensitive,all]. Mark those as `TypeEnum` with `Enum` set. (Hard-code this small map in the generator; it's the one place enum-typed strings are encoded.)
  - `isSecretKey(key)`: key ∈ {api_token, dsn, session_secret, auth_token} or contains `secret`/`password`/`private_key`.
  - `label(key)`: title-case the yaml key with spaces (`fetch_proxy`→"Fetch Proxy").
  - `advancedOnly`: true when type is `opaque`, or the field's go type has a custom `UnmarshalYAML` (hard-code the two: `WatchPath`, `LearnLockEnvironment`).
  - Walk `Config` recursively to build the `[]Field` tree (group fields recurse into their struct's fields). Skip unexported fields and `yaml:"-"`.
  - Defaults: instantiate `config.Defaults()`, reflect-walk by the same paths, set `Default` for scalar/bool/list/enum leaves (skip groups; for tristate, the default is the comment-documented value — parse `nil = ... (true|false)` from the doc comment, else omit).
  - Count leaves into `FieldCount`. Marshal indented JSON to `descriptor.json` (in the parent dir).

  Keep `classify`, `isSecretKey`, `label`, and the enum map as top-level funcs/vars so the test can call them.

- [ ] **Step 4: Run** `go test ./internal/config/configschema/gen/` → PASS.

- [ ] **Step 5: Commit**
```bash
gofumpt -w internal/config/configschema/gen/ && golangci-lint run ./internal/config/configschema/gen/
git add internal/config/configschema/gen/
git commit -m "feat(configschema): AST generator — classify, enums, defaults, flags"
```

---

### Task 3: Generate the real descriptor + coverage test + CI sync check

**Files:**
- Modify: `internal/config/configschema/descriptor.json` (generated)
- Modify: `internal/config/configschema/descriptor_test.go` (add coverage test)
- Modify: `Makefile` (add `configschema` generate + a `configschema-check`)

- [ ] **Step 1: Generate**
Run: `cd internal/config/configschema && go run ./gen` → overwrites `descriptor.json`.

- [ ] **Step 2: Add the coverage test** to `descriptor_test.go`:
```go
func TestDescriptorCoversWholeSchema(t *testing.T) {
	d, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	// Count leaves (non-group fields) in the descriptor tree.
	var leaves int
	var walk func(fs []Field)
	walk = func(fs []Field) {
		for i := range fs {
			if fs[i].Type == TypeGroup {
				walk(fs[i].Children)
			} else {
				leaves++
			}
		}
	}
	walk(d.Sections)
	if leaves != d.FieldCount || leaves == 0 {
		t.Fatalf("leaf count %d != FieldCount %d", leaves, d.FieldCount)
	}
	// Spot-check known fields exist with the right type.
	if d.Help("enforce") == "" {
		t.Error("enforce should have help text")
	}
}
```

- [ ] **Step 3: Run** `go test ./internal/config/configschema/` → PASS. Eyeball `descriptor.json`: confirm `mode` is enum, `enforce` is tristate, `fetch_proxy` is a group with `monitoring.blocklist` as a list, `kill_switch.api_token` is `secret:true`, `suppress` is `opaque`/advanced.

- [ ] **Step 4: Makefile targets** — add:
```make
configschema:
	cd internal/config/configschema && go run ./gen

configschema-check: configschema
	git diff --exit-code internal/config/configschema/descriptor.json
```

- [ ] **Step 5: Commit**
```bash
git add internal/config/configschema/descriptor.json internal/config/configschema/descriptor_test.go Makefile
git commit -m "feat(configschema): generate full descriptor + coverage + sync check"
```
(CI wiring for `configschema-check` is added in the final task.)

---

## Phase 2 — Node patch + value map

### Task 4: YAML-node patcher — set/insert/delete by path

**Files:**
- Create: `internal/console/configstructured/patch.go`
- Test: `internal/console/configstructured/patch_test.go`

- [ ] **Step 1: Write the failing test**
```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package configstructured

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func mustPatch(t *testing.T, src, path string, val any, help string) string {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	if err := ApplyChange(&doc, path, val, help); err != nil {
		t.Fatalf("ApplyChange(%s): %v", path, err)
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestApplyChange(t *testing.T) {
	const src = "mode: audit\nfetch_proxy:\n  listen: \"127.0.0.1:8888\"   # keep\n"

	// Replace an existing top-level scalar, preserving the rest.
	out := mustPatch(t, src, "mode", "strict", "")
	if !strings.Contains(out, "mode: strict") || !strings.Contains(out, "# keep") {
		t.Errorf("replace failed:\n%s", out)
	}

	// Insert a brand-new nested field under an existing parent (no duplicate parent).
	out = mustPatch(t, src, "fetch_proxy.timeout_seconds", 30, "Per-request timeout.")
	if strings.Count(out, "fetch_proxy:") != 1 {
		t.Errorf("duplicate parent:\n%s", out)
	}
	if !strings.Contains(out, "timeout_seconds: 30") {
		t.Errorf("insert failed:\n%s", out)
	}
	if !strings.Contains(out, "# Per-request timeout.") {
		t.Errorf("head comment missing:\n%s", out)
	}

	// Insert creating a brand-new top-level section.
	out = mustPatch(t, src, "kill_switch.api_listen", "127.0.0.1:9090", "")
	if !strings.Contains(out, "kill_switch:") || !strings.Contains(out, "api_listen: 127.0.0.1:9090") {
		t.Errorf("new section failed:\n%s", out)
	}

	// Delete (revert to default) removes the key.
	out = mustPatch(t, src, "mode", deleteSentinel, "")
	if strings.Contains(out, "mode:") {
		t.Errorf("delete failed:\n%s", out)
	}
}

func TestApplyChangeBool(t *testing.T) {
	out := mustPatch(t, "enforce: true\n", "enforce", false, "")
	if !strings.Contains(out, "enforce: false") {
		t.Errorf("bool replace:\n%s", out)
	}
}
```

- [ ] **Step 2: Run** `go test ./internal/console/configstructured/` → FAIL.

- [ ] **Step 3: Implement** `patch.go`. Key logic over `yaml.v3` nodes:
  - The unmarshaled `*yaml.Node` is a `DocumentNode` with one child `MappingNode`. Mapping children alternate key,value,key,value.
  - `deleteSentinel` is a package-level sentinel (`var deleteSentinel = &struct{}{}` exported or an exported const string `"\x00__delete__"`; use an exported `DeleteSentinel any`).
  - `ApplyChange(doc, path, val, help)`: split path on `.`; navigate/create nested mapping nodes for all but the last segment (insert a new `MappingNode` value if a parent key is missing); for the last segment: find the key node; if found and `val == DeleteSentinel`, remove the key+value pair from the parent mapping's `Content`; if found, replace the value node (keep its position; update via `scalarNode(val)`); if not found, append a new key scalar (with `HeadComment = help` when non-empty) + value scalar.
  - `scalarNode(val any) *yaml.Node`: build a `ScalarNode` whose `Value`/`Tag`/`Style` match the type (bool→`!!bool`, int→`!!int`, float→`!!float`, string→`!!str`; quote strings only when needed — mimic the Phase-1 `renderValue` rule). For list/map values use `yaml.Node`-encode via `node.Encode(val)`.
  - Guard: return an error if a path segment traverses through a non-mapping node (e.g. trying to nest under a scalar).

- [ ] **Step 4: Run** `go test ./internal/console/configstructured/ -run TestApplyChange -v` → PASS.

- [ ] **Step 5: Commit**
```bash
gofumpt -w internal/console/configstructured/ && golangci-lint run ./internal/console/configstructured/
git add internal/console/configstructured/patch.go internal/console/configstructured/patch_test.go
git commit -m "feat(configstructured): surgical yaml-node patch by path (set/insert/delete + head-comments)"
```

---

### Task 5: Effective values map + present-paths + secret redaction

**Files:**
- Create: `internal/console/configstructured/values.go`
- Test: `internal/console/configstructured/values_test.go`

- [ ] **Step 1: Write the failing test**
```go
func TestEffectiveValuesAndPresent(t *testing.T) {
	const raw = "mode: audit\nkill_switch:\n  api_token: \"super-secret\"\n"
	eff, err := EffectiveValues([]byte(raw), []string{"kill_switch.api_token"})
	if err != nil {
		t.Fatal(err)
	}
	if eff["mode"] != "audit" {
		t.Errorf("mode = %v", eff["mode"])
	}
	ks, _ := eff["kill_switch"].(map[string]any)
	if ks == nil || ks["api_token"] == "super-secret" {
		t.Errorf("api_token not redacted: %v", ks)
	}
	if ks["api_token"] != RedactedSentinel {
		t.Errorf("want redacted sentinel, got %v", ks["api_token"])
	}

	present := PresentPaths([]byte(raw))
	if !present["mode"] || !present["kill_switch.api_token"] {
		t.Errorf("present paths wrong: %v", present)
	}
	if present["fetch_proxy.listen"] {
		t.Error("absent path reported present")
	}
}
```

- [ ] **Step 2: Run** → FAIL.

- [ ] **Step 3: Implement** `values.go`:
  - `RedactedSentinel = "__redacted__"` (exported const).
  - `EffectiveValues(raw, secretPaths)`: `cfg := &config.Config{}`; `yaml.Unmarshal(raw, cfg)` (ignore err? no — return it); `cfg.ApplyDefaults()`; marshal back to yaml then unmarshal into `map[string]any` (gives a nested map keyed by yaml tags, with defaults filled). Walk `secretPaths` and replace each present leaf with `RedactedSentinel`. Return the map.
  - `PresentPaths(raw)`: unmarshal raw into `map[string]any`; recurse, recording every leaf and intermediate dotted path into a `map[string]bool`.

- [ ] **Step 4: Run** → PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/console/configstructured/values.go internal/console/configstructured/values_test.go
git commit -m "feat(configstructured): effective-values map, present-paths, secret redaction"
```

---

### Task 6: Default-preservation invariant test (the headline safety test)

**Files:**
- Test: `internal/console/configstructured/preserve_test.go`

- [ ] **Step 1: Write the test** — a config relying on defaults round-trips through parse→(no edits)→marshal with identical security semantics, and editing one field never disturbs tri-states:
```go
func TestNoEditPreservesSemantics(t *testing.T) {
	// enforce omitted (nil=true default), internal omitted (nil=SSRF on), scan_content omitted.
	const raw = "mode: balanced\nfetch_proxy:\n  monitoring:\n    blocklist:\n      - \"*.pastebin.com\"\n"
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatal(err)
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatal(err)
	}
	// No edits → byte-identical (node round-trip is lossless).
	if strings.TrimSpace(string(out)) != strings.TrimSpace(raw) {
		t.Errorf("round-trip changed file:\n--- got ---\n%s\n--- want ---\n%s", out, raw)
	}
	// Editing one field must not introduce enforce/internal/scan_content.
	if err := ApplyChange(&doc, "mode", "strict", ""); err != nil {
		t.Fatal(err)
	}
	out2, _ := yaml.Marshal(&doc)
	for _, k := range []string{"enforce:", "internal:", "scan_content:"} {
		if strings.Contains(string(out2), k) {
			t.Errorf("edit leaked %q into config:\n%s", k, out2)
		}
	}
}

func TestEditedConfigStillValidates(t *testing.T) {
	const raw = "mode: audit\n"
	var doc yaml.Node
	_ = yaml.Unmarshal([]byte(raw), &doc)
	_ = ApplyChange(&doc, "kill_switch.api_listen", "127.0.0.1:9090", "")
	out, _ := yaml.Marshal(&doc)
	if _, err := config.ValidateBytes(out); err != nil {
		t.Fatalf("patched config failed validation: %v\n%s", err, out)
	}
}
```

- [ ] **Step 2: Run** → PASS (proves the safety property). If `TestNoEditPreservesSemantics` shows yaml.v3 reflows the file, relax the assertion to "contains the blocklist and contains no enforce/internal/scan_content" — the security property (no leaked tri-states) is the must-pass part.

- [ ] **Step 3: Commit**
```bash
git add internal/console/configstructured/preserve_test.go
git commit -m "test(configstructured): default-preservation + validate-after-edit invariants"
```

---

## Phase 3 — Endpoints

### Task 7: GET /api/config/schema + GET /api/config/values

**Files:**
- Modify: `internal/console/server/server.go`
- Test: `internal/console/server/server_test.go`

- [ ] **Step 1: Write the failing test** (`TestConfigSchemaAndValuesEndpoints`) — model on the existing login→cookie pattern; assert: both require auth (401 without cookie); `GET /api/config/schema` returns JSON with non-zero `field_count`; `GET /api/config/values` returns `{effective, present}` and the `kill_switch.api_token` value (if any) is the redacted sentinel, never plaintext. Seed the configsvc temp file with `mode: audit\nkill_switch:\n  api_token: "tok"\n`.

- [ ] **Step 2: Run** → FAIL (routes missing).

- [ ] **Step 3: Implement** — add imports (`configschema`, `configstructured`) and a package-level `secretPaths` slice derived once from the descriptor (walk fields where `Secret`). Register:
```go
mux.Handle("GET /api/config/schema", d.Auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	desc, err := configschema.Load()
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	writeJSON(w, desc)
})))
mux.Handle("GET /api/config/values", d.Auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	raw, err := d.Config.Read()
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	eff, err := configstructured.EffectiveValues(raw, secretPaths)
	if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
	writeJSON(w, map[string]any{"effective": eff, "present": configstructured.PresentPaths(raw)})
})))
```
Compute `secretPaths` via a small helper that walks the descriptor once (memoize in a `sync.Once`).

- [ ] **Step 4: Run** → PASS.

- [ ] **Step 5: Commit**
```bash
gofumpt -w internal/console/server/ && golangci-lint run ./internal/console/server/
git add internal/console/server/
git commit -m "feat(console): GET /api/config/schema and /api/config/values (secret-redacted)"
```

---

### Task 8: POST /api/config/structured

**Files:**
- Modify: `internal/console/server/server.go`
- Test: `internal/console/server/server_test.go`

- [ ] **Step 1: Write the failing test** (`TestConfigStructuredEndpoint`): authed; body `{"changes":{"kill_switch.api_listen":"127.0.0.1:9090"}}` → 204 and the temp config file now contains `api_listen: 127.0.0.1:9090`; a change that produces invalid config (`{"changes":{"mode":"bogus"}}`) → 400/422 and the file is unchanged; unauth → 401; oversized body → rejected.

- [ ] **Step 2: Run** → FAIL.

- [ ] **Step 3: Implement** the route:
```go
mux.Handle("POST /api/config/structured", d.Auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	var body struct{ Changes map[string]any `json:"changes"` }
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest); return
	}
	raw, err := d.Config.Read()
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	patched, err := configstructured.ApplyChanges(raw, body.Changes, descHelpLookup)
	if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
	if err := d.Config.Write(patched); err != nil {
		var invalid *configsvc.InvalidConfigError
		if errors.As(err, &invalid) { http.Error(w, err.Error(), http.StatusBadRequest); return }
		http.Error(w, err.Error(), http.StatusInternalServerError); return
	}
	w.WriteHeader(http.StatusNoContent)
})))
```
Add `configstructured.ApplyChanges(raw []byte, changes map[string]any, help func(path string) string) ([]byte, error)` (a thin wrapper: unmarshal raw → node, loop `ApplyChange` over sorted paths with `help(path)`, marshal). `descHelpLookup` wraps the loaded descriptor's `Help`. `d.Config.Write` already validates (Phase-1 `configsvc.Write` → `Validate`), so validate-before-write is preserved.

- [ ] **Step 4: Run** → PASS. Also run `go test -race ./internal/console/...`.

- [ ] **Step 5: Commit**
```bash
git add internal/console/server/ internal/console/configstructured/
git commit -m "feat(console): POST /api/config/structured — sparse patch, validate-before-write"
```

---

## Phase 4 — Frontend

### Task 9: API client additions

**Files:** Modify `internal/console/web/app/src/api.ts`.

- [ ] **Step 1: Add types + calls** (mirror existing wrappers):
```ts
export interface SchemaField {
  path: string; key: string; label: string
  type: 'group' | 'bool' | 'tristate' | 'int' | 'float' | 'string' | 'enum' | 'list' | 'map' | 'opaque'
  help?: string; default?: unknown; enum?: string[]; secret?: boolean; advanced_only?: boolean
  children?: SchemaField[]
}
export interface ConfigSchema { field_count: number; sections: SchemaField[] }
export interface ConfigValues { effective: Record<string, unknown>; present: Record<string, boolean> }

export async function getConfigSchema(): Promise<ConfigSchema> {
  return asJSON<ConfigSchema>(await request('/api/config/schema'))
}
export async function getConfigValues(): Promise<ConfigValues> {
  return asJSON<ConfigValues>(await request('/api/config/values'))
}
export async function applyConfigStructured(changes: Record<string, unknown>): Promise<void> {
  const res = await request('/api/config/structured', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ changes }),
  })
  if (!res.ok) throw new ApiError(res.status, await res.text())
}
export const REDACTED_SENTINEL = '__redacted__'
```

- [ ] **Step 2:** `cd internal/console/web/app && npx tsc -b` → clean.
- [ ] **Step 3: Commit** `git add src/api.ts && git commit -m "feat(console): schema/values/structured api client"`.

---

### Task 10: `Field` — type-dispatching renderer + Vitest for value helpers

**Files:**
- Create: `internal/console/web/app/src/screens/config/Field.tsx`
- Create: `internal/console/web/app/src/screens/config/fieldvalue.ts` (pure: read a dotted path from the effective map; format/parse per type)
- Test: `internal/console/web/app/src/screens/config/fieldvalue.test.ts`

- [ ] **Step 1: Vitest for the pure helpers** (`fieldvalue.test.ts`): `getPath(effective, "fetch_proxy.monitoring.max_url_length")` returns the nested value; `getPath` of an absent path returns `undefined`; `coerce('int', "30")===30`, `coerce('bool', true)===true`, `coerce('float',"4.5")===4.5`.

- [ ] **Step 2: Run** `npx vitest run src/screens/config/fieldvalue.test.ts` → FAIL.

- [ ] **Step 3: Implement** `fieldvalue.ts` (`getPath(obj, dotted)`, `coerce(type, raw)`), then `Field.tsx` — a component `{ field: SchemaField, value: unknown, present: boolean, onChange(path, value) }` that renders by `field.type`:
  - `bool` → toggle (reuse the Config.tsx toggle style)
  - `tristate` → 3-segment control `Default (<default>) | On | Off`; "Default" emits a delete (sends `REDACTED?` no — sends a special `__default__` marker the parent maps to delete) — simplest: parent passes `onReset(path)`; Default button calls it
  - `enum` → `<select>` of `field.enum`
  - `int`/`float` → number input (coerce on change); `string` → text; `secret` → password input with placeholder "•• set ••" when `value===REDACTED_SENTINEL` (typing replaces; empty = leave unchanged)
  - `list` → reuse `ListEditor` (but operating on the structured patch, not YAML — adapt: a simple chips editor emitting the full array)
  - `map` → key/value rows
  - `opaque`/`advanced_only` → read-only note "Edit in the Advanced (raw YAML) view" + a link that switches views
  - always show `field.help` + default + an "overridden" badge when `present`

- [ ] **Step 4:** `npx vitest run` + `npx tsc -b` + `npm run lint` → clean.
- [ ] **Step 5: Commit** the two files + test.

---

### Task 11: `SectionTree` — nav + global search

**Files:** Create `internal/console/web/app/src/screens/config/SectionTree.tsx`.

- [ ] **Step 1: Implement** — props `{ sections: SchemaField[], selected: string, onSelect(path), query, onQuery }`. Renders the top-level sections as a nav list; a search box filters: when `query` non-empty, show a flat list of matching leaf fields (match on path/label/help, case-insensitive) across all sections, each clickable to select its section + scroll to it. This is the discoverability surface.
- [ ] **Step 2:** `npx tsc -b && npm run lint` → clean.
- [ ] **Step 3: Commit.**

---

### Task 12: `AllSettings` — compose tree + fields + save

**Files:** Create `internal/console/web/app/src/screens/config/AllSettings.tsx`.

- [ ] **Step 1: Implement** — on mount fetch `getConfigSchema()` + `getConfigValues()`. State: `selectedSection`, `query`, and a `changes: Record<string,unknown>` sparse patch (path→new value; a reset sets a `__delete__` marker). Render `SectionTree` (left) + the selected section's `Field`s (right), each bound to `getPath(values.effective, field.path)` overlaid with any pending `changes`. A sticky Save bar shows N pending changes; **Save** calls `applyConfigStructured(changes)` (mapping `__delete__` markers to the structured delete representation the backend understands — send a `null`-style sentinel the backend treats as delete), then re-fetches values and clears `changes`. Surface validation errors (400 body) in a banner. Reuse the toast + Banner components.
- [ ] **Step 2:** `npx tsc -b && npm run lint && npm run build` → clean.
- [ ] **Step 3: Commit.**

(Backend note: the structured endpoint must accept a delete marker. In Task 8, treat a change value equal to the JSON `null` OR a `{"__delete__":true}` object as `DeleteSentinel`. Add a tiny test for the delete path there if not already covered.)

---

## Phase 5 — Wire + verify

### Task 13: Replace Guided view with All Settings

**Files:** Modify `internal/console/web/app/src/screens/Config.tsx`.

- [ ] **Step 1: Implement** — change the view switch from `guided | advanced` to `settings | advanced` (default `settings`). Render `<AllSettings .../>` for `settings`; keep the Advanced raw editor branch unchanged; keep the unblock entry/dialog available (move the "allow a destination…" button into the AllSettings header or keep it global). Remove the three Phase-1 `ListEditor` instances and the guided panel (the generated tree replaces them). Keep `applyBuffer` only if still used by Advanced; otherwise remove dead code.
- [ ] **Step 2:** `npx tsc -b && npm run lint && npm run build` → clean.
- [ ] **Step 3: Commit** `git commit -m "feat(console): replace guided view with generated All Settings tree"`.

---

### Task 14: Full verification + CI sync wiring + docs

**Files:** Modify `.github/workflows/ci.yaml` (add `configschema-check`); modify `docs/console.md`.

- [ ] **Step 1: Add CI descriptor-sync check** — in the lint job (or a small step), run `make configschema-check` so a schema change without a regenerated descriptor fails CI. (Edit `.github/workflows/ci.yaml`; static literal step, no untrusted input.)
- [ ] **Step 2: Docs** — add an "All Settings" subsection to `docs/console.md`: the settings tree covers every config field, search to find a knob, tri-states show their default, secrets are masked, save validates before applying and hot-reloads; the raw editor remains for advanced/opaque fields.
- [ ] **Step 3: Full suite**
```bash
make configschema-check
golangci-lint run ./...
go test -race -count=1 ./internal/config/configschema/... ./internal/console/...
cd internal/console/web/app && npm test && npm run lint && npx tsc -b && npm run build
```
Expected: all green; descriptor in sync.
- [ ] **Step 4: Manual smoke** — run the console against `testing/`, open Config → All Settings, search "ssrf", toggle a tri-state to Off and back to Default, set `kill_switch.api_listen`, Save, confirm `pipelock.yaml` gained only that line (with a head-comment) and omitted sections/`enforce` are untouched.
- [ ] **Step 5: Commit + open PR** against `main`.

---

## Self-Review notes

- **Spec coverage:** descriptor+generator (Tasks 1–3) ✓; node-surgical save preserving omitted sections (Task 4) ✓; values + redaction + present (Task 5) ✓; default-preservation invariant (Task 6) ✓; 3 endpoints (Tasks 7–8) ✓; type-dispatched renderer + tree + search (Tasks 10–11) ✓; replace Guided, keep unblock + raw (Task 13) ✓; descriptor-sync CI (Task 14) ✓; secrets redacted + enterprise out-of-scope recorded in header ✓.
- **Known risk / acceptance:** yaml.v3 node round-trip may reflow whitespace/quote style on save (it is not a byte-perfect formatter). The *security* property (no leaked tri-states, omitted sections stay omitted, comments on existing nodes preserved) is what the tests assert; cosmetic reflow of edited files is acceptable and the raw editor remains for hand-formatting. Flagged here rather than silently assumed.
- **Type consistency:** Go `FieldType` string values match the TS `SchemaField.type` union exactly (group/bool/tristate/int/float/string/enum/list/map/opaque). `RedactedSentinel="__redacted__"` (Go) == `REDACTED_SENTINEL` (TS). Delete marker handled consistently in Task 8 + Task 12.
- **Decomposition:** one plan, five phases; Phases 1–2 (Go foundation) are independently testable before any UI exists.
