# pipelock-console — Guided Configuration (Phase 1)

Status: approved design, pre-implementation
Date: 2026-06-03
Scope: pipelock-console web app (`internal/console/...`)

## Problem

The console's **Config** screen ([internal/console/web/app/src/screens/Config.tsx](../../../internal/console/web/app/src/screens/Config.tsx))
is a raw YAML editor with two quick-toggles (`mode`, `enforce`). To change anything
else, an operator must know the schema and hand-edit YAML.

The concrete pain: an agent tried to reach a legitimate internal/private resource
and pipelock blocked it (SSRF / internal-IP layer). The operator saw a blocked
event but had **no obvious, safe way to allow that destination** — the fix lives in
`ssrf.ip_allowlist` (or `api_allowlist` / `blocklist`, depending on why it was
blocked), which requires knowing the schema.

The value this feature delivers: let an operator **understand why something was
blocked and fix it safely in a few clicks**, without learning the YAML schema —
while never weakening the security model.

## Goal (Phase 1)

1. **"Unblock a destination" recipe** — from a blocked event (or a standalone form),
   propose the *minimal* correct config change to allow a destination, explain what
   it relaxes and what still gets scanned, show a diff, validate, apply.
2. **A few guided forms** — structured editors for the highest-value list/section
   config: `api_allowlist`, `fetch_proxy.monitoring.blocklist`, `ssrf.ip_allowlist`,
   plus `kill_switch` basics and `emit.webhook` fields.
3. **Advanced (raw YAML) mode retained** — today's editor, unchanged, as the escape
   hatch for the long tail.

Out of scope (later phases): structured forms for all ~30 sections; schema-driven
metadata endpoint; per-pattern DLP editing.

## Mechanism mapping (the core security decision)

Block results already carry a structured `Reason` and an actionable `Hint`
([internal/scanner/scanner.go:127](../../../internal/scanner/scanner.go)). The minimal
fix per reason:

| Block reason | Minimal fix | Why content scanning still runs |
|---|---|---|
| SSRF / internal IP | add `<ip>/32` to `ssrf.ip_allowlist` | DLP is layer 5; SSRF is layer 8. The allowlist only skips layer 8. |
| Domain blocklist hit | remove the matching pattern from `fetch_proxy.monitoring.blocklist` | Blocklist is a destination gate, not a content gate. |
| Not in allowlist (strict mode) | add domain to `api_allowlist` | Allowlist controls reachability, not scanning. |

`ssrf.ip_allowlist` is purpose-built for this — the scanner's own hint says
*"add X to ssrf.ip_allowlist to allow this internal IP"*
([internal/scanner/core.go:682](../../../internal/scanner/core.go)).

**Implementation note (reconciled during build):** the strict-mode allowlist-miss
has no dedicated `blockreason` code (it falls through to `parse_error` in
`reasonFromScanner`), so the one-click unblock recipe covers `ssrf_private_ip`,
`ssrf_metadata`, and `domain_blocklist` only. `api_allowlist` is therefore edited
through the guided **ListEditor** rather than the auto-unblock path. The Events
"Allow this…" action is gated to exactly the three supported reasons, so it never
offers to "allow" a DLP/secret-leak, prompt-injection, or tool-policy block (for
which "allow" is not a safe or meaningful operation).

**Minimality guardrail:** for SSRF the proposal emits a single-host `/32` (or the exact
blocked CIDR), never a broad range. If only a broad change is possible, the proposal
returns a `warning` that the UI must surface and the operator must explicitly confirm.

## Architecture (Approach A: backend owns mechanism, frontend applies surgically)

```
blocked event (target, reason, hint)
        │  "Allow this…"
        ▼
POST /api/config/unblock-proposal  {target, reason?}
        │  backend maps reason → minimal mechanism (reuses block-reason source of truth)
        ▼
{ op, path, value, explanation, stillScanned[], warning? }
        │  frontend applies op to the YAML *text* via yamlpatch (comments preserved)
        ▼
diff preview + explanation + "what still gets scanned"  →  operator confirms
        ▼
POST /api/config/validate  →  POST /api/config  →  proxy hot-reloads
```

Rationale: the backend is already the source of truth for the schema, validation,
and the canonical block-reason mapping, so the one security-critical decision
("what is the *minimal* safe change?") lives in testable Go. The frontend never
re-serializes the file (your commented `pipelock.yaml` survives intact) — it makes
surgical text edits, exactly like the existing scalar quick-toggles, then routes
through the **existing** validate→apply→hot-reload path.

### Backend

New small package `internal/console/configintents` (keeps `configsvc` focused):

- `Proposal` type: `{Op string; Path string; Value string; Explanation string; StillScanned []string; Warning string}`.
  `Op` ∈ {`list_add`, `list_remove`}.
- `ProposeUnblock(target, reason string) (Proposal, error)` — pure function mapping
  `(target, reason)` → minimal `Proposal`. Normalizes `target` (extracts host/IP from a
  URL; computes `/32` for a bare IP). Maps reason via the canonical block-reason
  constants, not string-matching free text. Returns an error for an unknown/unsupported
  reason (fail closed — no proposal rather than a guess).

New endpoint in [server.go](../../../internal/console/server/server.go), behind `RequireAuth`,
`MaxBytesReader`-limited:

- `POST /api/config/unblock-proposal` — decodes `{target, reason}`, calls
  `ProposeUnblock`, returns the `Proposal` as JSON. Never writes config; pure
  read/compute. (Apply still goes through the existing `POST /api/config`.)

### Frontend

- **`lib/yamlpatch.ts`** — extend with comment-preserving sequence helpers, same
  line-based philosophy as the existing scalar helpers (no full YAML parse):
  - `readSequence(yaml, path): string[]` — items under a (possibly nested) key.
  - `addToSequence(yaml, path, value): string` — idempotent; creates the key/section
    if absent; preserves surrounding lines/comments.
  - `removeFromSequence(yaml, path, value): string` — no-op if absent.
  - Supports the nested path `fetch_proxy.monitoring.blocklist` and `ssrf.ip_allowlist`.
- **`screens/Config.tsx`** — add a **Guided** / **Advanced** view switch. Advanced is
  today's CodeMirror editor + quick-toggles, unchanged. Guided hosts the list editors
  and section fields, each routing through the existing `applyQuickToggle`-style
  validate→apply flow (generalized to operate on the patched buffer).
- **`screens/config/` components:**
  - `ListEditor` — add/remove chips for a sequence key (used by allowlist, blocklist,
    ssrf.ip_allowlist). Reads current items via `yamlpatch.readSequence`.
  - `UnblockDialog` — destination input (or prefilled), calls the proposal endpoint,
    renders explanation + `stillScanned` + `warning` (if any) + diff, gated **Confirm**
    that applies the op and runs validate→apply.
- **`screens/Events.tsx`** — an **"Allow this…"** action on blocked events, prefilling
  `target` + `reason` into `UnblockDialog`. If the event carries no structured block
  reason, `UnblockDialog` shows a small block-type selector (SSRF / blocklist /
  allowlist) so the mapping stays deterministic rather than guessed. The first
  implementation task must confirm which reason/target fields the emitted event
  actually carries (see `internal/proxy` emit path) and wire the prefill to those.
- **`api.ts`** — `proposeUnblock(target, reason?)` call + `Proposal` type.

## Security invariants (must be enforced and tested)

- **Validate-before-write unchanged.** Every apply still goes through
  `configsvc.Validate` via `POST /api/config`. The new endpoint only *proposes*.
- **Proposals are minimal.** SSRF → `/32` (or exact blocked CIDR); never a broad range.
  Broad-only changes return a `warning` and require explicit operator confirmation.
- **No bypass of content scanning.** Explanations always state what is relaxed and that
  DLP/content scanning still runs. No "fully trusted / unscanned" framing anywhere.
- **Severity stays non-editable.** No UI for per-event severity; only thresholds.
- **Fail closed.** Unknown/unsupported block reason → error + no proposal, not a guess.
- **Auth + limits.** New endpoint behind `RequireAuth` with `MaxBytesReader`.

## Testing

**Go (`configintents`):**
- Table-driven `ProposeUnblock`: each supported reason → expected `{op, path, value}`,
  including: URL→host normalization, bare-IP→`/32`, the minimality guardrail, the
  broad-change `warning`, and unknown-reason → error.
- `StillScanned`/`Explanation` are populated (no empty security copy).

**Go (`server`):**
- `POST /api/config/unblock-proposal` requires auth; returns the proposal JSON;
  rejects oversized bodies; bad input → 400.
- Existing validate-before-write path still rejects invalid applied config.

**TS (`yamlpatch`):**
- `addToSequence`/`removeFromSequence` round-trip: preserves unrelated lines and
  comments; idempotent add; no-op remove; creates a missing nested section correctly.
- `readSequence` returns items for top-level and nested keys.

**TS (flow):**
- `UnblockDialog` renders proposal + `stillScanned`; **Confirm is gated** (cannot apply
  without it); a `warning` is shown when present.
- Parity: an Events-triggered unblock produces the same proposal/apply as the standalone
  form for the same `(target, reason)`.

## Boundaries / files

- Backend: `internal/console/configintents/` (new); one route in
  `internal/console/server/server.go`; `api.ts` client call.
- Frontend: `internal/console/web/app/src/lib/yamlpatch.ts` (extended);
  `screens/Config.tsx` (Guided/Advanced); `screens/config/ListEditor.tsx`,
  `screens/config/UnblockDialog.tsx` (new); `screens/Events.tsx` ("Allow this…").

## Non-goals

- No new proxy/scanner config *capabilities* — this is a UI/UX layer over the existing
  schema, validation, and block-reason mapping.
- No auto-apply — every write is operator-confirmed.
- No re-serialization of `pipelock.yaml` — edits are surgical to preserve comments.
