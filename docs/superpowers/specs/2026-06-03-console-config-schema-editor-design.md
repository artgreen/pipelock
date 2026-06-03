# pipelock-console — Complete Schema-Driven Configuration Editor

Status: approved design, pre-implementation
Date: 2026-06-03
Scope: pipelock-console web app + a schema descriptor generated from `internal/config`

## Problem

pipelock's config has **71 top-level sections and 566 individual fields** (`internal/config/schema.go`, 1,758 lines). Operators can't discover what knobs exist; when something blocks unexpectedly, they have no way to find the relevant setting short of reading the schema source. The Phase-1 console (merged in #2) added an "unblock a destination" recipe + three hand-coded list editors + a raw YAML editor — useful, but it covers a tiny fraction of the surface.

Goal: a **traditional, complete settings interface** — every field, logically grouped into a navigable tree/pages, searchable — so the full set of "knobs and levers" is visible and editable. No wizards.

## Decisions (locked with the user)

1. **Schema-driven (generated).** The UI is generated from the Go config structs so it covers all 566 fields by construction and stays in sync. Generic-but-consistent field rendering is the accepted tradeoff for completeness.
2. **Surgical YAML-node patch on save, with auto-comments on new fields.** Saving parses `pipelock.yaml` into a `yaml.v3` node tree (preserving structure and existing comments), sets/inserts only the changed paths, attaches the schema's help text as a head-comment on **newly added** fields, then validates and writes. (Refined from "whole re-serialize" during planning: a full marshal would expand every omitted section to explicit zero-values and could flip section-level defaults — unsafe. Node-surgical patching preserves omitted sections and existing comments.) The raw editor remains for hand-tuning.
3. **The generated "All Settings" view replaces the Guided view.** The three Phase-1 list editors are absorbed into the generated tree. The **unblock recipe** and the **raw Advanced editor** are kept.
4. **Sparse-patch save model** (the security core — see below).

## The security landmine this design is built around

The schema encodes behavior in *absence*:
- **31 tri-state `*bool` fields** where `nil` ≠ `false`. `nil` means a specific default, often `true` (e.g. `Enforce` nil=true, `ScanContent` nil=true, `IncludeDefaults` nil=true). CLAUDE.md hard rule: omitting the field must produce the default.
- **`Internal []string`** where `nil` disables SSRF checks entirely (vs `[]` = no internal ranges).
- A naive "show effective values, write them all back" would silently convert these from "track the default" to a **pinned explicit value** — flipping security semantics. Unacceptable.

Therefore the editor never rewrites unedited fields. It tracks only the fields the operator actually changes and applies just those onto the **as-loaded** config (nils intact) before re-serializing. **Untouched tri-states and `nil` slices survive byte-for-byte in meaning.** This is the headline invariant, proven by test.

## Architecture (Approach A: build-time codegen from the schema AST)

```
internal/config/schema.go ──(go:generate AST walk)──► descriptor.json (committed, CI-verified in sync)
                                                            │
console backend:                                            ▼
  GET  /api/config/schema   ── serves descriptor ──► tree of {path,key,type,default,help,enum?,secret?,triState?,advancedOnly?}
  GET  /api/config/values   ── load raw cfg ──► { effective: <defaults-applied, for display>, present: <set-mask> }
  POST /api/config/structured ── sparse {path:value} patch ──► apply onto RAW cfg ──► serialize w/ comments ──► Validate() ──► write
                                                            │
frontend "All Settings" view:                              ▼
  fetch schema + values → render section tree (searchable) → edit fields → Save → validate → apply → hot-reload
```

### 1. Schema descriptor — `internal/config/configschema` (new) + generator

A generator (`go:generate`, run via `make`) parses `internal/config/schema.go`'s **AST** (go/ast can read struct fields, yaml tags, AND doc comments — runtime reflection cannot) and walks the `Config` type tree to emit a descriptor. For each field:
- `path` (dotted, e.g. `fetch_proxy.monitoring.blocklist`), `key` (yaml), `label`
- `type`: `bool` | `tristate` | `int` | `string` | `enum` | `list` | `map` | `group`
- `default`: from `config.Defaults()` (reflection at gen time)
- `help`: the field's Go doc comment (the authors' own prose)
- `enum`: for fields whose type is a known enum (`config.Action*`, `config.Mode*`, `config.Severity*` const blocks, extracted via AST)
- flags: `secret` (api_token, dsn, …), `triState` (`*bool` / nil-sensitive), `advancedOnly`
- The two custom-`UnmarshalYAML` types (`WatchPath`, `LearnLockEnvironment`) are marked `advancedOnly` — they have no safe structured round-trip, so they're edited via the raw editor only and shown read-only in the tree with a "edit in raw" link.

The descriptor is committed (`internal/config/configschema/descriptor.json`) and embedded; a CI check regenerates and diffs to guarantee it never drifts from the schema.

### 2. Backend endpoints (authed, in `internal/console/server`)

- `GET /api/config/schema` → the embedded descriptor (static).
- `GET /api/config/values` → loads the current `pipelock.yaml` and returns:
  - `effective`: the config with `ApplyDefaults()` applied, marshaled to JSON, **with secret fields redacted** (returned as a sentinel like `"•••set"` or empty, never plaintext).
  - `present`: a set of dotted paths that are explicitly present in the file (so the UI can badge "default" vs "overridden").
- `POST /api/config/structured` → body `{ changes: { "<path>": <value>, ... } }` (sparse). Backend:
  1. Parse the on-disk `pipelock.yaml` into a `yaml.v3` document **node tree** (preserves structure + comments; absent sections stay absent).
  2. For each changed path: navigate the node tree by yaml key; if the field exists, replace its value node; if absent, insert the key (creating missing parent maps), and attach the descriptor's help as a `HeadComment`. A "revert to default" change deletes the field's node. Secret sentinel values mean "unchanged — leave the existing node untouched."
  3. Marshal the node tree back to bytes.
  4. `ValidateBytes()` (full fidelity) → on failure return 422 with the validation message.
  5. Write via the existing atomic temp+rename path in `configsvc`.

  This is server-side surgical patching over the YAML AST — the typed `Config` struct is never re-marshaled, so omitted fields/sections and unset tri-states are physically preserved.

### 3. Frontend — "All Settings" view (`screens/config/`)

- Replaces the Guided view in `Config.tsx` (Advanced raw editor + unblock entry remain).
- **Left nav**: a tree of the 71 sections (top-level → nested groups), with a **search box** that filters fields across the whole schema by path/label/help — the core discoverability feature.
- **Right pane**: the selected section's fields, rendered by `type`:
  - `bool` → toggle; `tristate` → three-state control showing "Default (true) / On / Off" so the default is visible and only overridden deliberately
  - `enum` → select; `int` → number; `string` → text; `secret` → masked input (set/replace, never displays the value)
  - `list` → reuse the Phase-1 `ListEditor`; `map` → key-value editor; `group` → nested sub-form
  - each field shows help text + default; "overridden" badge when `present`
- Edits tracked as a **sparse `{path: value}` patch**; **Save** posts it → validate → apply → hot-reload; validation errors surfaced inline by path.

### Reuse from Phase 1
`configsvc` (validate + atomic write), the validate→apply→hot-reload flow, `ListEditor`, the unblock recipe (`configintents`, `UnblockDialog`, the "allow this…" event action), and the raw editor all carry over unchanged.

## Security & correctness invariants (must be proven by tests)

- **Default preservation (headline):** load a config that relies on defaults (omitted `enforce`, omitted `scan_content`, `internal` absent) → open + save with **zero edits** → the written file's parsed semantics are identical (nils stay nil; SSRF still enabled-by-default; enforce still default). No tri-state is pinned.
- **Targeted change only:** editing one field changes only that field's line(s); all others (including comments-from-schema for untouched fields) are deterministic and unrelated fields keep their values.
- **Validate-before-write** preserved (server chokepoint). Invalid edits fail closed, nothing persisted.
- **Secrets never leave the backend in plaintext** via `/api/config/values`; a secret left untouched in the UI is written back unchanged (sentinel → keep existing).
- **Descriptor sync:** CI regenerates the descriptor and fails if it differs from the committed copy.
- **advancedOnly fields** are never serialized by the structured path (only via raw editor).

## Components / boundaries

- `internal/config/configschema/` (new): descriptor type, the `go:generate` generator (AST walk over `schema.go`), embedded `descriptor.json`, and a path→help lookup (for `HeadComment` on newly inserted nodes). Lives next to the schema it mirrors.
- `internal/console/server/server.go`: three new authed routes.
- `internal/console/configstructured` (new): the YAML-node patcher — parse bytes → `yaml.Node`; `applyChange(doc, path, value)` (navigate/replace/insert/delete nodes, head-comment new fields); marshal back. Plus the `/values` value-map builder (yaml→map of the defaults-applied config, secret-path redaction, present-paths set from the raw file). Calls `config.ValidateBytes` + `configsvc` atomic write. Keeps `configsvc` focused on raw read/validate/write.
- Frontend: `screens/config/AllSettings.tsx`, `screens/config/Field.tsx` (type-dispatching renderer), `screens/config/SectionTree.tsx`, `api.ts` additions; `Config.tsx` swaps Guided → All Settings.

## Testing

- **Go (configschema):** generator golden test asserting the descriptor covers every yaml-tagged field (count == 566, no field missing); enum extraction; default extraction; `advancedOnly`/`secret`/`triState` flagging.
- **Go (round-trip):** the default-preservation suite across the 31 tri-states + `Internal` nil + a representative scalar/list/map; sparse-patch apply + serialize + reload equals expected; validate-before-write; secret redaction + keep-existing.
- **Go (server):** auth on all three routes; values redacts secrets; structured rejects invalid → 422.
- **Frontend:** `Field` renders correctly per type (incl. tri-state three-state + secret mask); search filters across sections; sparse-patch tracking; save flow + inline validation errors.

## Non-goals / phasing

- Not changing any proxy/scanner config *capability* — purely surfacing the existing schema.
- No per-field bespoke widgets in v1 beyond the type-based set (generated + consistent). A curated overlay for high-traffic sections is a possible later phase.
- `advancedOnly` fields (2 custom-unmarshaler types) are raw-editor-only in v1.
- Enterprise (`enterprise/`, build-tagged) fields: descriptor covers the OSS schema; enterprise-only fields are out of scope for v1 unless trivially included.

## Build order (for the plan)

1. `configschema` descriptor type + AST generator + golden test (the foundation).
2. Commented-YAML serializer + sparse-patch apply on raw config + round-trip/default-preservation tests.
3. Backend endpoints (schema, values w/ redaction, structured write) + tests.
4. Frontend type-dispatching `Field` + `SectionTree` + search.
5. Wire `AllSettings` into `Config.tsx` replacing Guided; save/validate flow.
6. Verification + docs.
