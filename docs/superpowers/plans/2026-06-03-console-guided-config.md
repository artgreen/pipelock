# Console Guided Configuration (Phase 1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let an operator allow a blocked destination and edit key list/section config from the pipelock-console UI, with the minimal safe change and content scanning preserved — no hand-editing YAML.

**Architecture:** The backend owns the one security-critical decision (block reason → minimal config change) as a pure, table-tested Go function exposed at `POST /api/config/unblock-proposal`. The frontend applies that change as a surgical, comment-preserving edit to the YAML *text* (extending the existing `yamlpatch` helpers), then routes it through the **existing** validate → apply → hot-reload path. The raw editor is retained as an "Advanced" view.

**Tech Stack:** Go 1.25+ (`net/http`, `internal/blockreason`), React 19 + TypeScript + Vite 8, Vitest (new, for pure-function tests).

Spec: [docs/superpowers/specs/2026-06-03-console-guided-config-design.md](../specs/2026-06-03-console-guided-config-design.md)

---

## File Structure

**Backend (Go):**
- Create `internal/console/configintents/intents.go` — `Proposal` type + `ProposeUnblock` pure function (the mechanism mapping).
- Create `internal/console/configintents/intents_test.go` — table-driven tests.
- Modify `internal/console/server/server.go` — register `POST /api/config/unblock-proposal`.
- Modify `internal/console/server/server_test.go` — endpoint tests.

**Frontend (TS/React):**
- Modify `internal/console/web/app/package.json` — add `vitest` + `test` script.
- Modify `internal/console/web/app/src/lib/yamlpatch.ts` — add sequence helpers.
- Create `internal/console/web/app/src/lib/yamlpatch.test.ts` — Vitest tests.
- Modify `internal/console/web/app/src/api.ts` — `proposeUnblock` + `Proposal` type.
- Create `internal/console/web/app/src/screens/config/ListEditor.tsx` — add/remove chips for a sequence key.
- Create `internal/console/web/app/src/screens/config/UnblockDialog.tsx` — proposal → explain → diff → confirm → apply.
- Modify `internal/console/web/app/src/screens/Config.tsx` — Guided/Advanced views; wire ListEditors + Unblock entry.
- Modify `internal/console/web/app/src/screens/Events.tsx` — "Allow this…" action on blocked events.

**Conventions to follow:** gofumpt; error wrapping `fmt.Errorf("…: %w", err)`; table tests with `t.Run`; `0o600`/`0o750` perms; `_, _ = fmt.Fprintf` / `_ = w.Write`; React files use the existing inline-style pattern seen in `Config.tsx`. Run `golangci-lint run ./...` before `go test`.

---

## Task 1: `configintents.ProposeUnblock` — the mechanism mapping

**Files:**
- Create: `internal/console/configintents/intents.go`
- Test: `internal/console/configintents/intents_test.go`

- [ ] **Step 1: Write the failing test**

`internal/console/configintents/intents_test.go`:
```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package configintents

import (
	"strings"
	"testing"

	"github.com/luckyPipewrench/pipelock/internal/blockreason"
)

func TestProposeUnblock(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		reason    string
		wantOp    string
		wantPath  string
		wantValue string
		wantWarn  bool
		wantErr   bool
	}{
		{"ssrf ipv4 -> /32", "10.1.2.3", string(blockreason.SSRFPrivateIP), OpListAdd, "ssrf.ip_allowlist", "10.1.2.3/32", false, false},
		{"ssrf url -> host /32", "http://10.1.2.3:9000/x", string(blockreason.SSRFPrivateIP), OpListAdd, "ssrf.ip_allowlist", "10.1.2.3/32", false, false},
		{"ssrf ipv6 -> /128", "fd00::5", string(blockreason.SSRFPrivateIP), OpListAdd, "ssrf.ip_allowlist", "fd00::5/128", false, false},
		{"ssrf existing cidr kept", "10.0.0.0/8", string(blockreason.SSRFPrivateIP), OpListAdd, "ssrf.ip_allowlist", "10.0.0.0/8", false, false},
		{"ssrf non-ip host errors", "internal.local", string(blockreason.SSRFPrivateIP), "", "", "", false, true},
		{"metadata warns", "169.254.169.254", string(blockreason.SSRFMetadata), OpListAdd, "ssrf.ip_allowlist", "169.254.169.254/32", true, false},
		{"blocklist remove host", "http://x.pastebin.com/raw", string(blockreason.DomainBlocklist), OpListRemove, "fetch_proxy.monitoring.blocklist", "x.pastebin.com", false, false},
		{"dns rebind refused", "1.2.3.4", string(blockreason.SSRFDNSRebind), "", "", "", false, true},
		{"unknown reason errors", "1.2.3.4", "totally_unknown", "", "", "", false, true},
		{"empty target errors", "", string(blockreason.SSRFPrivateIP), "", "", "", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ProposeUnblock(tt.target, tt.reason)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got proposal %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Op != tt.wantOp || got.Path != tt.wantPath || got.Value != tt.wantValue {
				t.Errorf("got {op:%q path:%q value:%q}, want {op:%q path:%q value:%q}", got.Op, got.Path, got.Value, tt.wantOp, tt.wantPath, tt.wantValue)
			}
			if (got.Warning != "") != tt.wantWarn {
				t.Errorf("warning presence = %v, want %v (warning=%q)", got.Warning != "", tt.wantWarn, got.Warning)
			}
			if strings.TrimSpace(got.Explanation) == "" {
				t.Error("explanation must not be empty")
			}
			if len(got.StillScanned) == 0 {
				t.Error("still_scanned must list remaining protections")
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/console/configintents/ -run TestProposeUnblock`
Expected: FAIL — package/`ProposeUnblock` undefined.

- [ ] **Step 3: Write minimal implementation**

`internal/console/configintents/intents.go`:
```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package configintents maps a blocked destination to the minimal, safe config
// change that would permit it. It only computes proposals; applying them goes
// through the console's existing validate->apply path.
package configintents

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/luckyPipewrench/pipelock/internal/blockreason"
)

// Op codes for a Proposal.
const (
	OpListAdd    = "list_add"
	OpListRemove = "list_remove"
)

// Config paths the unblock recipe can touch.
const (
	PathSSRFIPAllowlist = "ssrf.ip_allowlist"
	PathBlocklist       = "fetch_proxy.monitoring.blocklist"
)

// Proposal is a minimal config change that allows a previously-blocked
// destination. It is computed, never applied here.
type Proposal struct {
	Op           string   `json:"op"`            // list_add | list_remove
	Path         string   `json:"path"`          // dotted config path
	Value        string   `json:"value"`         // item to add/remove
	Explanation  string   `json:"explanation"`   // plain-language description
	StillScanned []string `json:"still_scanned"` // protections that remain active
	Warning      string   `json:"warning,omitempty"`
}

// ProposeUnblock maps a (target, reason) to the minimal config change that
// permits the destination. Fails closed: unknown/unsupported reasons return an
// error rather than guessing.
func ProposeUnblock(target, reason string) (Proposal, error) {
	host := normalizeTarget(target)
	if host == "" {
		return Proposal{}, fmt.Errorf("empty or unparseable target %q", target)
	}
	switch blockreason.Reason(reason) {
	case blockreason.SSRFPrivateIP:
		cidr, err := hostToCIDR(host)
		if err != nil {
			return Proposal{}, err
		}
		return Proposal{
			Op:    OpListAdd,
			Path:  PathSSRFIPAllowlist,
			Value: cidr,
			Explanation: fmt.Sprintf("Adds %s to ssrf.ip_allowlist, exempting only this address from the private-IP / SSRF block.", cidr),
			StillScanned: []string{
				"DLP secret scanning (runs before the SSRF layer)",
				"prompt-injection / response scanning",
				"domain blocklist",
			},
		}, nil
	case blockreason.SSRFMetadata:
		cidr, err := hostToCIDR(host)
		if err != nil {
			return Proposal{}, err
		}
		return Proposal{
			Op:           OpListAdd,
			Path:         PathSSRFIPAllowlist,
			Value:        cidr,
			Explanation:  fmt.Sprintf("Adds %s to ssrf.ip_allowlist. This address is a cloud instance-metadata endpoint.", cidr),
			StillScanned: []string{"DLP secret scanning", "prompt-injection / response scanning"},
			Warning: "This is a cloud instance-metadata address (e.g. 169.254.169.254). Allowing it is a common SSRF " +
				"escalation path and can expose cloud credentials. Only proceed if you specifically intend to allow metadata access.",
		}, nil
	case blockreason.DomainBlocklist:
		return Proposal{
			Op:    OpListRemove,
			Path:  PathBlocklist,
			Value: host,
			Explanation: fmt.Sprintf("Removes the matching pattern from fetch_proxy.monitoring.blocklist so %s is no longer blocked at the domain layer.", host),
			StillScanned: []string{
				"DLP secret scanning",
				"prompt-injection / response scanning",
				"SSRF / private-IP checks",
			},
		}, nil
	case blockreason.SSRFDNSRebind:
		return Proposal{}, fmt.Errorf("reason %q has no safe minimal allow: the host resolved to a private IP after appearing public; allowing it would defeat the rebinding protection", reason)
	default:
		return Proposal{}, fmt.Errorf("unsupported block reason %q for one-click unblock", reason)
	}
}

// normalizeTarget extracts a host or IP from a URL, host:port, or bare host.
func normalizeTarget(target string) string {
	t := strings.TrimSpace(target)
	if t == "" {
		return ""
	}
	if strings.Contains(t, "://") {
		if u, err := url.Parse(t); err == nil && u.Hostname() != "" {
			return u.Hostname()
		}
	}
	if h, _, err := net.SplitHostPort(t); err == nil && h != "" {
		return h
	}
	return t
}

// hostToCIDR turns a bare IP into a single-host CIDR; passes through an existing
// CIDR; errors on a non-IP host (SSRF blocks are on resolved IPs).
func hostToCIDR(host string) (string, error) {
	if _, _, err := net.ParseCIDR(host); err == nil {
		return host, nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return "", fmt.Errorf("SSRF allow needs an IP address, got %q", host)
	}
	if ip.To4() != nil {
		return ip.String() + "/32", nil
	}
	return ip.String() + "/128", nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/console/configintents/ -run TestProposeUnblock -v`
Expected: PASS (all subtests).

- [ ] **Step 5: Lint + commit**

```bash
golangci-lint run ./internal/console/configintents/
git add internal/console/configintents/
git commit -m "feat(console): config-unblock proposal mapping (reason -> minimal safe change)"
```

---

## Task 2: `POST /api/config/unblock-proposal` endpoint

**Files:**
- Modify: `internal/console/server/server.go`
- Test: `internal/console/server/server_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/console/server/server_test.go` (follow the existing login→cookie pattern used in `e2e_test.go`; this test builds the handler, logs in, then calls the endpoint):
```go
func TestUnblockProposalEndpoint(t *testing.T) {
	hash, _ := auth.HashPassword("pw")
	h := server.New(server.Deps{
		Auth:    auth.NewManager(auth.Options{PasswordHash: hash, SecretHex: "00112233445566778899aabbccddeeff"}),
		Config:  configsvc.New(t.TempDir() + "/pipelock.yaml"),
		Client:  pipelockclient.New(pipelockclient.Options{BaseURL: "http://127.0.0.1:1"}),
		Service: service.New("pipelock"),
		Buffer:  events.NewBuffer(10),
		Hub:     events.NewHub(),
	})
	ts := httptest.NewServer(h)
	defer ts.Close()
	client := ts.Client()

	// Unauthenticated → 401.
	unauth, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/api/config/unblock-proposal", strings.NewReader(`{"target":"10.1.2.3","reason":"ssrf_private_ip"}`))
	ur, err := client.Do(unauth)
	if err != nil {
		t.Fatal(err)
	}
	_ = ur.Body.Close()
	if ur.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauth status = %d, want 401", ur.StatusCode)
	}

	// Log in to get a session cookie.
	lr, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/api/login", strings.NewReader(`{"password":"pw"}`))
	lresp, err := client.Do(lr)
	if err != nil {
		t.Fatal(err)
	}
	_ = lresp.Body.Close()
	cookie := lresp.Cookies()[0]

	do := func(bodyStr string) *http.Response {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/api/config/unblock-proposal", strings.NewReader(bodyStr))
		req.AddCookie(cookie)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	// Supported reason → 200 with proposal.
	ok := do(`{"target":"10.1.2.3","reason":"ssrf_private_ip"}`)
	defer func() { _ = ok.Body.Close() }()
	if ok.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", ok.StatusCode)
	}
	var prop configintents.Proposal
	if err := json.NewDecoder(ok.Body).Decode(&prop); err != nil {
		t.Fatal(err)
	}
	if prop.Op != configintents.OpListAdd || prop.Path != configintents.PathSSRFIPAllowlist || prop.Value != "10.1.2.3/32" {
		t.Errorf("unexpected proposal: %+v", prop)
	}

	// Unsupported reason → 422.
	bad := do(`{"target":"1.2.3.4","reason":"nope"}`)
	_ = bad.Body.Close()
	if bad.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("unsupported reason status = %d, want 422", bad.StatusCode)
	}
}
```
Add imports as needed: `encoding/json`, `github.com/luckyPipewrench/pipelock/internal/console/configintents` (and confirm `auth`, `configsvc`, `events`, `pipelockclient`, `service`, `httptest`, `net/http`, `strings` are present — they are in `e2e_test.go`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/console/server/ -run TestUnblockProposalEndpoint`
Expected: FAIL — route returns 404/405; `configintents` import unused until wired.

- [ ] **Step 3: Add the route**

In `internal/console/server/server.go`, add the import `"github.com/luckyPipewrench/pipelock/internal/console/configintents"`, then register the route alongside the other `/api/config*` handlers (e.g. right after `POST /api/config/validate`):
```go
mux.Handle("POST /api/config/unblock-proposal", d.Auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Target string `json:"target"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	prop, err := configintents.ProposeUnblock(body.Target, body.Reason)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, prop)
})))
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/console/server/ -run TestUnblockProposalEndpoint -v`
Expected: PASS.

- [ ] **Step 5: Lint + commit**

```bash
golangci-lint run ./internal/console/server/
git add internal/console/server/
git commit -m "feat(console): POST /api/config/unblock-proposal endpoint"
```

---

## Task 3: Frontend test harness (Vitest)

**Files:**
- Modify: `internal/console/web/app/package.json`

- [ ] **Step 1: Add Vitest as a dev dependency and a test script**

Edit `internal/console/web/app/package.json`:
- In `"scripts"`, add `"test": "vitest run"`.
- In `"devDependencies"`, add `"vitest": "^3.2.4"`.

Resulting `scripts` block:
```json
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "lint": "eslint .",
    "preview": "vite preview",
    "test": "vitest run"
  },
```

- [ ] **Step 2: Install**

Run: `cd internal/console/web/app && npm install`
Expected: lockfile updates, `vitest` resolves.

- [ ] **Step 3: Smoke-test the runner**

Run: `cd internal/console/web/app && npx vitest run --reporter=dot`
Expected: exits 0 with "No test files found" (acceptable) — confirms the runner works.

- [ ] **Step 4: Commit**

```bash
git add internal/console/web/app/package.json internal/console/web/app/package-lock.json
git commit -m "chore(console): add vitest for frontend unit tests"
```

---

## Task 4: `yamlpatch` sequence helpers (surgical, comment-preserving)

**Files:**
- Modify: `internal/console/web/app/src/lib/yamlpatch.ts`
- Test: `internal/console/web/app/src/lib/yamlpatch.test.ts`

These edit block sequences under a top-level or one-level-nested key, preserving all other lines/comments. Four add sub-cases must be handled: (a) item already present (no-op), (b) list exists → insert item, (c) parent section exists but key absent → insert key+item, (d) nothing exists → append a new block.

- [ ] **Step 1: Write the failing tests**

`internal/console/web/app/src/lib/yamlpatch.test.ts`:
```ts
import { describe, expect, it } from 'vitest'
import { readSequence, addToSequence, removeFromSequence } from './yamlpatch'

const nested = `mode: audit
fetch_proxy:
  listen: "127.0.0.1:8888"
  monitoring:
    max_url_length: 2048
    blocklist:
      - "*.pastebin.com"   # keep this comment
      - "*.file.io"
forward_proxy:
  enabled: true
`

describe('readSequence', () => {
  it('reads a nested sequence', () => {
    expect(readSequence(nested, 'fetch_proxy.monitoring.blocklist')).toEqual(['*.pastebin.com', '*.file.io'])
  })
  it('returns [] for an absent key', () => {
    expect(readSequence(nested, 'ssrf.ip_allowlist')).toEqual([])
  })
})

describe('removeFromSequence', () => {
  it('removes an item and preserves the rest + comments', () => {
    const out = removeFromSequence(nested, 'fetch_proxy.monitoring.blocklist', '*.file.io')
    expect(readSequence(out, 'fetch_proxy.monitoring.blocklist')).toEqual(['*.pastebin.com'])
    expect(out).toContain('# keep this comment')
    expect(out).toContain('forward_proxy:')
  })
  it('is a no-op for an absent item', () => {
    expect(removeFromSequence(nested, 'fetch_proxy.monitoring.blocklist', 'nope')).toBe(nested)
  })
})

describe('addToSequence', () => {
  it('appends to an existing list and is idempotent', () => {
    const once = addToSequence(nested, 'fetch_proxy.monitoring.blocklist', '*.evil.com')
    expect(readSequence(once, 'fetch_proxy.monitoring.blocklist')).toContain('*.evil.com')
    expect(addToSequence(once, 'fetch_proxy.monitoring.blocklist', '*.evil.com')).toBe(once)
  })
  it('creates a brand-new top-level section when absent', () => {
    const out = addToSequence(nested, 'ssrf.ip_allowlist', '10.1.2.3/32')
    expect(readSequence(out, 'ssrf.ip_allowlist')).toEqual(['10.1.2.3/32'])
    // unrelated content preserved
    expect(out).toContain('mode: audit')
    expect(out).toContain('# keep this comment')
  })
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd internal/console/web/app && npx vitest run src/lib/yamlpatch.test.ts`
Expected: FAIL — `readSequence`/`addToSequence`/`removeFromSequence` are not exported.

- [ ] **Step 3: Implement the helpers**

Append to `internal/console/web/app/src/lib/yamlpatch.ts`:
```ts
// ─── Sequence (list) helpers ─────────────────────────────────────────────────
// Surgical, line-based edits of block sequences under a top-level or
// single-nested key (e.g. "api_allowlist" or "fetch_proxy.monitoring.blocklist").
// Consistent with the scalar helpers: no full YAML parse; the backend validates.

type SeqBlock = { keyLine: number; keyIndent: number; firstItem: number; endItem: number }

function leadingSpaces(line: string): number {
  const m = line.match(/^(\s*)/)
  return m ? m[1].length : 0
}

function stripComment(s: string): string {
  return s.replace(/\s+#.*$/, '')
}

function unquote(s: string): string {
  const v = s.trim()
  if ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'"))) return v.slice(1, -1)
  return v
}

// Locate the key line for a dotted path by walking segments with increasing
// indent. Returns undefined if any segment is missing.
function findKeyLine(lines: string[], path: string): number | undefined {
  const segs = path.split('.')
  let start = 0
  let parentIndent = -1
  let found = -1
  for (let s = 0; s < segs.length; s++) {
    const re = new RegExp(`^(\\s*)${escapeKey(segs[s])}:`)
    found = -1
    for (let i = start; i < lines.length; i++) {
      const m = lines[i].match(re)
      if (!m) continue
      const indent = m[1].length
      if (indent <= parentIndent) continue
      found = i
      parentIndent = indent
      start = i + 1
      break
    }
    if (found === -1) return undefined
  }
  return found
}

function locateSeq(yaml: string, path: string): SeqBlock | undefined {
  const lines = yaml.split('\n')
  const keyLine = findKeyLine(lines, path)
  if (keyLine === undefined) return undefined
  const keyIndent = leadingSpaces(lines[keyLine])
  let firstItem = -1
  let endItem = keyLine
  for (let i = keyLine + 1; i < lines.length; i++) {
    const line = lines[i]
    if (line.trim() === '') {
      endItem = i
      continue
    }
    const indent = leadingSpaces(line)
    if (indent <= keyIndent) break
    if (/^\s*-\s+/.test(line)) {
      if (firstItem === -1) firstItem = i
      endItem = i
    } else if (line.trim().startsWith('#')) {
      endItem = i
    } else {
      break
    }
  }
  return { keyLine, keyIndent, firstItem, endItem }
}

export function readSequence(yaml: string, path: string): string[] {
  const loc = locateSeq(yaml, path)
  if (!loc || loc.firstItem === -1) return []
  const lines = yaml.split('\n')
  const items: string[] = []
  for (let i = loc.firstItem; i <= loc.endItem; i++) {
    const m = lines[i].match(/^\s*-\s+(.*)$/)
    if (m) items.push(unquote(stripComment(m[1])))
  }
  return items
}

export function addToSequence(yaml: string, path: string, value: string): string {
  if (readSequence(yaml, path).includes(value)) return yaml // idempotent
  const lines = yaml.split('\n')
  const loc = locateSeq(yaml, path)
  const rendered = renderValue(value)
  if (loc) {
    const itemIndent = ' '.repeat(loc.keyIndent + 2)
    lines.splice(loc.endItem + 1, 0, `${itemIndent}- ${rendered}`)
    return lines.join('\n')
  }
  // Section/key absent: append a new top-level block (only single-nested paths).
  const segs = path.split('.')
  const trailingNL = yaml.endsWith('\n') ? '' : '\n'
  let block = trailingNL
  for (let i = 0; i < segs.length - 1; i++) block += `${'  '.repeat(i)}${segs[i]}:\n`
  const leafIndent = '  '.repeat(segs.length - 1)
  block += `${leafIndent}${segs[segs.length - 1]}:\n${leafIndent}  - ${rendered}\n`
  return yaml + block
}

export function removeFromSequence(yaml: string, path: string, value: string): string {
  const loc = locateSeq(yaml, path)
  if (!loc || loc.firstItem === -1) return yaml
  const lines = yaml.split('\n')
  for (let i = loc.firstItem; i <= loc.endItem; i++) {
    const m = lines[i].match(/^\s*-\s+(.*)$/)
    if (m && unquote(stripComment(m[1])) === value) {
      lines.splice(i, 1)
      return lines.join('\n')
    }
  }
  return yaml
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd internal/console/web/app && npx vitest run src/lib/yamlpatch.test.ts`
Expected: PASS (all). Also run `npm run lint` to confirm no eslint errors.

- [ ] **Step 5: Commit**

```bash
git add internal/console/web/app/src/lib/yamlpatch.ts internal/console/web/app/src/lib/yamlpatch.test.ts
git commit -m "feat(console): comment-preserving yaml sequence add/remove/read helpers"
```

---

## Task 5: API client — `proposeUnblock`

**Files:**
- Modify: `internal/console/web/app/src/api.ts`

- [ ] **Step 1: Add the type and call**

Add to the wire-types section of `api.ts`:
```ts
export interface UnblockProposal {
  op: 'list_add' | 'list_remove'
  path: string
  value: string
  explanation: string
  still_scanned: string[]
  warning?: string
}
```
Add near the other endpoint wrappers (after `applyConfig`):
```ts
// proposeUnblock asks the backend for the minimal config change that allows a
// blocked destination. Returns the proposal; 422 (unsupported reason) throws
// ApiError with the reason in the body.
export async function proposeUnblock(target: string, reason: string): Promise<UnblockProposal> {
  const res = await request('/api/config/unblock-proposal', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ target, reason }),
  })
  return asJSON<UnblockProposal>(res)
}
```

- [ ] **Step 2: Type-check**

Run: `cd internal/console/web/app && npx tsc -b`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/console/web/app/src/api.ts
git commit -m "feat(console): proposeUnblock api client + UnblockProposal type"
```

---

## Task 6: `ListEditor` component

**Files:**
- Create: `internal/console/web/app/src/screens/config/ListEditor.tsx`

A controlled list editor: shows current items as removable chips + an input to add one. It does **not** apply directly; it calls back with the new YAML buffer (parent owns validate/apply). Uses `yamlpatch` to compute edits.

- [ ] **Step 1: Implement the component**

`internal/console/web/app/src/screens/config/ListEditor.tsx`:
```tsx
import { useState } from 'react'
import { addToSequence, readSequence, removeFromSequence } from '../../lib/yamlpatch'

interface Props {
  label: string
  help: string
  path: string
  buffer: string
  disabled?: boolean
  onChange: (nextBuffer: string) => void
}

export default function ListEditor({ label, help, path, buffer, disabled, onChange }: Props) {
  const [draft, setDraft] = useState('')
  const items = readSequence(buffer, path)

  const add = () => {
    const v = draft.trim()
    if (!v) return
    onChange(addToSequence(buffer, path, v))
    setDraft('')
  }

  return (
    <div className="panel" style={{ marginBottom: '0.9rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: '1rem' }}>
        <span style={{ fontSize: '0.7rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'var(--color-text)' }}>{label}</span>
        <code style={{ color: 'var(--color-muted)', fontSize: '0.62rem' }}>{path}</code>
      </div>
      <p style={{ color: 'var(--color-muted)', fontSize: '0.72rem', margin: '0.35rem 0 0.6rem' }}>{help}</p>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.4rem', marginBottom: '0.6rem' }}>
        {items.length === 0 && <span style={{ color: 'var(--color-muted)', fontSize: '0.72rem' }}>— empty —</span>}
        {items.map((it) => (
          <span key={it} style={chip}>
            {it}
            <button type="button" disabled={disabled} onClick={() => onChange(removeFromSequence(buffer, path, it))} style={chipX} aria-label={`remove ${it}`}>×</button>
          </span>
        ))}
      </div>
      <div style={{ display: 'flex', gap: '0.4rem' }}>
        <input
          value={draft}
          disabled={disabled}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && add()}
          placeholder="add an entry…"
          style={{ flex: 1, background: 'var(--color-bg)', border: '1px solid var(--color-border)', color: 'var(--color-text)', padding: '0.35rem 0.5rem', fontFamily: 'var(--font-mono)', fontSize: '0.74rem' }}
        />
        <button type="button" className="btn-neon" disabled={disabled || !draft.trim()} onClick={add}>add</button>
      </div>
    </div>
  )
}

const chip: React.CSSProperties = {
  display: 'inline-flex', alignItems: 'center', gap: '0.4rem',
  border: '1px solid var(--color-border)', borderRadius: 'var(--radius-panel)',
  padding: '0.2rem 0.5rem', fontFamily: 'var(--font-mono)', fontSize: '0.72rem', color: 'var(--color-text)',
}
const chipX: React.CSSProperties = {
  background: 'none', border: 'none', color: 'var(--color-alert)', cursor: 'pointer', fontSize: '0.9rem', lineHeight: 1, padding: 0,
}
```

- [ ] **Step 2: Type-check**

Run: `cd internal/console/web/app && npx tsc -b`
Expected: no errors. (Component is wired in Task 8.)

- [ ] **Step 3: Commit**

```bash
git add internal/console/web/app/src/screens/config/ListEditor.tsx
git commit -m "feat(console): ListEditor component for sequence config keys"
```

---

## Task 7: `UnblockDialog` component

**Files:**
- Create: `internal/console/web/app/src/screens/config/UnblockDialog.tsx`

Given a `target` (and optional `reason`), fetch the proposal, render explanation + `still_scanned` + `warning`, require a **Confirm** click, then apply the op to the buffer and run the parent's validate→apply.

- [ ] **Step 1: Implement the component**

`internal/console/web/app/src/screens/config/UnblockDialog.tsx`:
```tsx
import { useEffect, useState } from 'react'
import { ApiError, proposeUnblock, type UnblockProposal } from '../../api'
import { addToSequence, removeFromSequence } from '../../lib/yamlpatch'

const REASONS = ['ssrf_private_ip', 'ssrf_metadata', 'domain_blocklist'] as const

interface Props {
  target: string
  reason?: string
  buffer: string
  onCancel: () => void
  // Applies the patched buffer through the parent's validate->apply path.
  onApply: (patched: string, summary: string) => Promise<void>
}

export default function UnblockDialog({ target, reason, buffer, onCancel, onApply }: Props) {
  const [tgt, setTgt] = useState(target)
  const [rsn, setRsn] = useState(reason ?? '')
  const [prop, setProp] = useState<UnblockProposal | null>(null)
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [applying, setApplying] = useState(false)

  const fetchProposal = async () => {
    setLoading(true)
    setErr(null)
    setProp(null)
    try {
      setProp(await proposeUnblock(tgt.trim(), rsn))
    } catch (e) {
      setErr(e instanceof ApiError ? e.body || e.message : e instanceof Error ? e.message : 'failed')
    } finally {
      setLoading(false)
    }
  }

  // Auto-fetch when we arrive with both fields prefilled (event-driven path).
  useEffect(() => {
    if (target.trim() && reason) void fetchProposal()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const confirm = async () => {
    if (!prop) return
    const patched = prop.op === 'list_add'
      ? addToSequence(buffer, prop.path, prop.value)
      : removeFromSequence(buffer, prop.path, prop.value)
    setApplying(true)
    try {
      await onApply(patched, `${prop.op === 'list_add' ? 'allow' : 'unblock'} ${prop.value}`)
    } finally {
      setApplying(false)
    }
  }

  return (
    <div style={overlay}>
      <div className="panel panel--neon" style={modal}>
        <h3 style={{ marginTop: 0, color: 'var(--color-neon)' }}>Allow a blocked destination</h3>

        <label style={lbl}>destination</label>
        <input value={tgt} onChange={(e) => setTgt(e.target.value)} style={field} placeholder="host, IP, or URL" />

        <label style={lbl}>block reason</label>
        <select value={rsn} onChange={(e) => setRsn(e.target.value)} style={field}>
          <option value="">— select —</option>
          {REASONS.map((r) => <option key={r} value={r}>{r}</option>)}
        </select>

        <div style={{ margin: '0.7rem 0' }}>
          <button type="button" className="btn-neon" disabled={!tgt.trim() || !rsn || loading} onClick={fetchProposal}>
            {loading ? '…checking' : 'preview change'}
          </button>
        </div>

        {err && <div style={{ color: 'var(--color-alert)', fontSize: '0.74rem', marginBottom: '0.6rem' }}>✕ {err}</div>}

        {prop && (
          <div className="panel" style={{ marginBottom: '0.7rem' }}>
            <p style={{ marginTop: 0, fontSize: '0.78rem' }}>{prop.explanation}</p>
            <div style={{ fontSize: '0.72rem', color: 'var(--color-muted)' }}>
              <strong>still scanned:</strong>
              <ul style={{ margin: '0.3rem 0 0', paddingLeft: '1.1rem' }}>
                {prop.still_scanned.map((s) => <li key={s}>{s}</li>)}
              </ul>
            </div>
            <pre style={diffLine}>+ {prop.path}: {prop.op === 'list_add' ? `add ${prop.value}` : `remove ${prop.value}`}</pre>
            {prop.warning && <div style={warnBox}>⚠ {prop.warning}</div>}
          </div>
        )}

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem' }}>
          <button type="button" className="btn-neon" onClick={onCancel} disabled={applying}>cancel</button>
          <button type="button" className="btn-alert" onClick={confirm} disabled={!prop || applying}>
            {applying ? '…applying' : 'confirm & apply'}
          </button>
        </div>
      </div>
    </div>
  )
}

const overlay: React.CSSProperties = { position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.6)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 50 }
const modal: React.CSSProperties = { width: 'min(560px, 92vw)', maxHeight: '88vh', overflow: 'auto', padding: '1.25rem' }
const lbl: React.CSSProperties = { display: 'block', fontSize: '0.62rem', letterSpacing: '0.1em', textTransform: 'uppercase', color: 'var(--color-muted)', margin: '0.5rem 0 0.2rem' }
const field: React.CSSProperties = { width: '100%', background: 'var(--color-bg)', border: '1px solid var(--color-border)', color: 'var(--color-text)', padding: '0.4rem 0.5rem', fontFamily: 'var(--font-mono)', fontSize: '0.78rem' }
const diffLine: React.CSSProperties = { background: 'rgba(57,255,20,0.08)', color: 'var(--color-neon)', padding: '0.4rem 0.6rem', fontSize: '0.74rem', margin: '0.5rem 0 0', whiteSpace: 'pre-wrap' }
const warnBox: React.CSSProperties = { marginTop: '0.5rem', border: '1px solid #5a5000', background: 'rgba(255,200,0,0.08)', color: 'var(--color-warn)', padding: '0.5rem 0.6rem', fontSize: '0.74rem' }
```

- [ ] **Step 2: Type-check**

Run: `cd internal/console/web/app && npx tsc -b`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/console/web/app/src/screens/config/UnblockDialog.tsx
git commit -m "feat(console): UnblockDialog — preview + confirm a destination allow"
```

---

## Task 8: Wire Guided view + ListEditors + Unblock entry into Config screen

**Files:**
- Modify: `internal/console/web/app/src/screens/Config.tsx`

Add a **Guided** / **Advanced** view toggle. Advanced = the existing editor (everything currently rendered). Guided = the ListEditors for the three sequence keys + a "Allow a blocked destination" button that opens `UnblockDialog`. Reuse the existing `validateConfig`/`applyConfig` flow via a shared `applyBuffer` helper.

- [ ] **Step 1: Add a shared apply helper and view state**

In `Config.tsx`, add near the other handlers a generalized apply that the Guided controls use (mirrors `applyQuickToggle` but takes a full buffer + summary):
```tsx
const applyBuffer = useCallback(async (patched: string, summary: string) => {
  setBuffer(patched)
  setApplying(true)
  try {
    const v = await validateConfig(patched)
    if (!v.ok) {
      setValidation(v)
      toast.push(`${summary} failed validation`, 'alert')
      return
    }
    await applyConfig(patched)
    toast.push(`${summary} — applied; pipelock will hot-reload`, 'ok')
    await load()
  } catch (e) {
    const reason = e instanceof ApiError ? e.body || e.message : e instanceof Error ? e.message : 'failed'
    setValidation({ ok: false, error: reason })
    toast.push(`${summary} rejected`, 'alert')
  } finally {
    setApplying(false)
  }
}, [load, toast])
```
Add state:
```tsx
const [view, setView] = useState<'guided' | 'advanced'>('guided')
const [unblockOpen, setUnblockOpen] = useState(false)
```
Add imports: `ListEditor from './config/ListEditor'`, `UnblockDialog from './config/UnblockDialog'`.

- [ ] **Step 2: Render the view switch + Guided panel**

Add a Guided/Advanced segmented control next to the existing buttons in `ScreenHeader right`, and render the Guided panel when `view === 'guided'` (wrap the current editor/diff grid so it shows only when `view === 'advanced'`):
```tsx
{view === 'guided' ? (
  <div style={{ flex: 1, minHeight: 0, overflow: 'auto' }}>
    <div className="panel" style={{ marginBottom: '0.9rem', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
      <span style={{ fontSize: '0.78rem' }}>A destination getting blocked? Allow it safely.</span>
      <button type="button" className="btn-neon" onClick={() => setUnblockOpen(true)} disabled={busy}>allow a destination…</button>
    </div>
    <ListEditor label="SSRF IP allowlist" path="ssrf.ip_allowlist" buffer={buffer} disabled={busy}
      help="Internal/private IPs to exempt from the SSRF block. Content (DLP) scanning still runs. CIDR form, e.g. 10.1.2.3/32."
      onChange={(next) => applyBuffer(next, 'ssrf.ip_allowlist change')} />
    <ListEditor label="Domain allowlist" path="api_allowlist" buffer={buffer} disabled={busy}
      help="In strict mode, only these domains are reachable. Does not bypass content scanning."
      onChange={(next) => applyBuffer(next, 'api_allowlist change')} />
    <ListEditor label="Domain blocklist" path="fetch_proxy.monitoring.blocklist" buffer={buffer} disabled={busy}
      help="Destination patterns that are always blocked, e.g. *.pastebin.com."
      onChange={(next) => applyBuffer(next, 'blocklist change')} />
  </div>
) : (
  /* existing editor + diff grid goes here, unchanged */
  <>{/* ...existing JSX... */}</>
)}
{unblockOpen && (
  <UnblockDialog
    target="" buffer={buffer}
    onCancel={() => setUnblockOpen(false)}
    onApply={async (patched, summary) => { await applyBuffer(patched, summary); setUnblockOpen(false) }}
  />
)}
```
Note: each `ListEditor.onChange` applies immediately via `applyBuffer` (validate+apply+reload), matching the existing quick-toggle UX.

- [ ] **Step 3: Type-check + lint + build**

Run: `cd internal/console/web/app && npx tsc -b && npm run lint`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/console/web/app/src/screens/Config.tsx
git commit -m "feat(console): guided config view with list editors + unblock entry"
```

---

## Task 9: "Allow this…" action on blocked events

**Files:**
- Modify: `internal/console/web/app/src/screens/Events.tsx`

On rows that represent a block, add an "Allow this…" button that opens `UnblockDialog` prefilled from the event's `fields`. **First confirm** what the emit path puts in `fields` (grep `internal/proxy` for the webhook/emit event construction and the keys used for target + block reason) and read from those keys; fall back to the dialog's reason selector when the reason field is absent.

- [ ] **Step 1: Confirm the event field names**

Run: `grep -rnE 'fields\["|"target"|"reason"|"block_reason"|Fields\[' internal/proxy/*.go internal/emit/*.go 2>/dev/null | grep -iE 'target|reason' | head`
Record the exact keys (e.g. `fields.target`, `fields.reason`). Use them in Step 2.

- [ ] **Step 2: Add the action + dialog**

In `Events.tsx`, import `UnblockDialog` and the config screen's apply path. Because `Events.tsx` does not own the config buffer, the dialog there must fetch the current config first (`getConfig`) and apply via `validateConfig`/`applyConfig`. Add a small handler:
```tsx
import { useState } from 'react'
import { applyConfig, getConfig, validateConfig } from '../api'
import UnblockDialog from './config/UnblockDialog'
// ...
const [unblock, setUnblock] = useState<{ target: string; reason: string } | null>(null)

// helper passed to UnblockDialog.onApply
const applyFromEvents = async (patched: string) => {
  const v = await validateConfig(patched)
  if (!v.ok) throw new Error(v.error || 'invalid config')
  await applyConfig(patched)
}
```
Render an "Allow this…" button on rows whose severity indicates a block (e.g. `severity === 'critical'` or a block `type`); prefer showing it whenever `fields.target` is present:
```tsx
{isBlockish(ev) && (ev.fields.target as string) && (
  <button type="button" className="btn-neon" onClick={() => setUnblock({ target: String(ev.fields.target), reason: String(ev.fields.reason ?? '') })}>
    allow this…
  </button>
)}
```
Render the dialog once, loading the current buffer lazily:
```tsx
{unblock && <UnblockGate target={unblock.target} reason={unblock.reason} onClose={() => setUnblock(null)} />}
```
Where `UnblockGate` is a tiny wrapper in the same file that loads the config buffer then renders `UnblockDialog`:
```tsx
function UnblockGate({ target, reason, onClose }: { target: string; reason: string; onClose: () => void }) {
  const [buf, setBuf] = useState<string | null>(null)
  useEffect(() => { void getConfig().then(setBuf).catch(() => setBuf('')) }, [])
  if (buf === null) return null
  return (
    <UnblockDialog
      target={target} reason={reason} buffer={buf}
      onCancel={onClose}
      onApply={async (patched) => {
        const v = await validateConfig(patched)
        if (!v.ok) throw new Error(v.error || 'invalid config')
        await applyConfig(patched)
        onClose()
      }}
    />
  )
}
```
Define `isBlockish(ev)` consistent with how the Events screen already styles BLOCK/WARN rows (reuse the existing severity/type check rather than inventing a new one).

- [ ] **Step 3: Type-check + lint + build the web app**

Run: `cd internal/console/web/app && npx tsc -b && npm run lint && npm run build`
Expected: clean build (this also regenerates `dist/` embedded by Go).

- [ ] **Step 4: Commit**

```bash
git add internal/console/web/app/src/screens/Events.tsx
git commit -m "feat(console): 'allow this' action on blocked events"
```

---

## Task 10: Full verification + spec reconciliation

**Files:**
- Modify: `docs/superpowers/specs/2026-06-03-console-guided-config-design.md` (one-line note)

- [ ] **Step 1: Reconcile the spec with the reality found in Task 1**

The strict-mode allowlist-miss has no dedicated block-reason code (it falls through to `parse_error`), so the auto-unblock recipe covers `ssrf_private_ip`, `ssrf_metadata`, and `domain_blocklist`; `api_allowlist` is edited via the guided ListEditor instead. Add a one-line note to the spec's mechanism section recording this.

- [ ] **Step 2: Run the whole backend + frontend test suite**

```bash
golangci-lint run ./...
go test -race -count=1 ./internal/console/...
cd internal/console/web/app && npm test && npm run lint && npx tsc -b && npm run build
```
Expected: all green; `dist/` rebuilt.

- [ ] **Step 3: Manual smoke (optional but recommended)**

Build both binaries, run with `testing/pipelock.yaml` + `testing/pipelock-console.yaml`, trigger a block (e.g. curl the proxy toward a private IP in `enforce` mode), then use "allow this…" on the event and confirm the destination is permitted and the YAML diff added `ssrf.ip_allowlist`.

- [ ] **Step 4: Commit + open PR**

```bash
git add docs/superpowers/specs/2026-06-03-console-guided-config-design.md
git commit -m "docs(console): note allowlist-miss has no reason code; handled via list editor"
```
Then push the branch and open a PR against `main` summarizing the guided-config Phase 1 feature.

---

## Self-Review notes

- **Spec coverage:** unblock recipe (Tasks 1,2,5,7,9) ✓; guided list forms (Tasks 4,6,8) ✓; advanced raw retained (Task 8) ✓; minimality + warning (Task 1) ✓; validate-before-write preserved (Tasks 2,8) ✓; auth + limits (Task 2) ✓; still-scanned messaging (Tasks 1,7) ✓; tests (Tasks 1,2,4 + Task 10) ✓.
- **Kill-switch / webhook guided fields:** the spec lists these as "a few guided forms" too, but Phase-1 value lives in the list editors + unblock recipe; kill-switch already has a dedicated screen and `emit.webhook` is low-churn. They are intentionally deferred to a Phase-2 follow-up to keep this plan focused (YAGNI). Recorded here rather than silently dropped.
- **Frontend component tests:** Phase 1 tests the pure `yamlpatch` helpers (highest bug risk) under Vitest; React component/DOM tests are deferred (would add `@testing-library/react`). The security-critical mapping is fully tested in Go.
- **Type consistency:** wire type `UnblockProposal` (TS) mirrors `Proposal` (Go) with snake_case json tags (`still_scanned`); `op` values `list_add`/`list_remove` match `OpListAdd`/`OpListRemove`; paths match `PathSSRFIPAllowlist`/`PathBlocklist`.
