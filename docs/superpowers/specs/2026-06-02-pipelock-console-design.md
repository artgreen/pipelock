# Pipelock Console — Design

**Date:** 2026-06-02
**Status:** Approved design, ready for implementation planning

## Summary

`pipelock-console` is a standalone web application for operating a local pipelock
instance from a browser, replacing the SSH-based `pipelock-tui`. It runs on the
same host as pipelock, owns the pipelock config file, proxies pipelock's runtime
data to the browser, and provides a single management surface for observing
traffic, editing config, flipping the kill switch, and restarting the service —
with no SSH anywhere.

It ships as a new binary built from the pipelock repository (`cmd/pipelock-console/`),
so it can import `internal/config` and reuse pipelock's real validation rather
than reimplementing it (the maintenance trap the old TUI fell into).

## Goals

- Operate pipelock from a browser over the network — no SSH, no terminal dependency.
- Full management in v1: **observe**, **edit config**, **kill switch**, **service control**.
- Beautiful, richly functional UI; single static binary to deploy.
- Reuse pipelock's config validation directly — zero validation drift.

## Non-Goals (v1)

- Managing multiple pipelock instances (single local instance only).
- Centralized/multi-host management, remote agents, SSH transport.
- Fine-grained RBAC / multi-user accounts (single admin password).
- Heavy auth hardening (lockout, rate-limit, mTLS, OIDC). One password gate is enough.

## Architecture

### Topology (single host)

```
Browser ──HTTPS──▶ pipelock-console ──localhost──▶ pipelock (HTTP API)
   (you)            (authed web app)       │
                          ├─ reads/writes pipelock.yaml  (validate → backup → atomic write)
                          ├─ controls systemd unit  (systemctl status/restart pipelock)
                          └─ receives pipelock's emitted events (webhook → /ingest)
```

The console runs on the same box as pipelock. The browser reaches the console's
web port over the network; the console reaches pipelock over localhost. TLS is
expected via the existing edge proxy (Traefik) or the console's own cert.

### Code home

- New binary: `cmd/pipelock-console/` inside the pipelock module
  (`github.com/luckyPipewrench/pipelock`).
- Imports `internal/config` for `Load`, `Validate`, `ValidateWithWarnings`,
  `ValidateReload`. Validation can never drift from the running pipelock.
- Standalone process, released/versioned alongside pipelock.

### Backend (Go)

Standard library `net/http`, minimal dependencies. Components:

- **Config service** — wraps the pipelock `config` package. Validates submitted
  YAML before writing. On success: timestamped backup → temp file → `fsync` →
  atomic `rename`. pipelock's fsnotify/SIGHUP picks up the change and hot-reloads
  (no restart needed).
- **pipelock client** — typed HTTP calls to pipelock's existing endpoints:
  `/health`, `/stats`, `/api/v1/sessions` (+ detail), `/api/v1/killswitch`
  (+ `/status`), `/api/v1/adaptive/*`. Degrades gracefully when pipelock is down.
- **Event sink** — a localhost `/ingest` endpoint that pipelock's `emit.webhook`
  POSTs to. Events (`{severity, type, timestamp, pipelock_instance, fields}`)
  land in an in-memory ring buffer.
- **Live stream** — Server-Sent Events push new events and refreshed stats to
  connected browsers; stats/sessions refresh on a poll interval.
- **Service controller** — runs `systemctl status/restart pipelock`. The console
  process runs with permission to do this; no extra privilege ceremony.
- **Auth** — single admin password (argon2id-hashed), session cookie. Sole
  purpose: keep agents/others out of the console.
- **Console config** — its own small YAML: web listen address, TLS settings,
  pipelock base URL(s), path to `pipelock.yaml`, admin password hash.

### Frontend

- Vite + React + Tailwind + shadcn/ui, built to static assets and embedded into
  the Go binary via `go:embed`. Node is a build-time-only dependency; the deploy
  artifact is a single static binary.
- **Visual direction: "Cyber terminal"** — dark, monospace, neon accents,
  scanline texture; leans into the security-tool identity.
- **Navigation:** left sidebar, persistent top bar (live pipelock status +
  kill-switch toggle).

### Screens

1. **Overview** — health, key counters (requests, blocked, flagged, active
   sessions), blocks-by-layer breakdown, recent live events at a glance.
2. **Events** — live streaming, filterable event table; row detail drawer.
3. **Sessions** — active sessions table; per-session detail.
4. **Config** — YAML editor with live validation (against the real config
   package), diff vs. current, backup list, write/apply; plus structured
   quick-toggles for the highest-value settings (mode/enforce, kill switch).
5. **Service** — systemd state, restart button, console + pipelock versions,
   reload history.

## Data Flow (the three write paths)

1. **Config edit:** browser submits YAML → console validates with the real
   `config.Validate` → on pass: backup + atomic write → pipelock hot-reloads
   automatically. On fail: reject, write nothing, return the exact error.
2. **Live events:** pipelock `emit.webhook` → console `/ingest` → ring buffer →
   SSE → browser feed.
3. **Kill switch / service:** UI action → console → pipelock killswitch API
   (instant) or local `systemctl restart` (for upgrades / wedged state).

## Security (lean)

- **One password gate.** Login → session cookie. Only goal: an agent or other
  party can't reach the console without the password. Password stored hashed.
  No lockout / rate-limit / constant-time / mTLS machinery.
- **Capability separation kept.** Console manages config and reads pipelock's
  API; holds no agent secrets. (Reading `pipelock.yaml` is config, not agent
  credentials.)
- **Validate before reload (firm).** Invalid config is never written.
- **Service control:** plain `systemctl status/restart pipelock`, no sudoers/
  polkit essay.
- **Event ingest:** localhost-only, kept simple.

## Error Handling

- Config validation failure → reject with exact error, nothing written (fail-closed).
- pipelock unreachable → degraded read-only UI with a clear banner, no crash.
- Service restart failure → surface stderr/exit status to the operator.
- If a config write cannot confirm a successful reload, show a warning rather
  than claiming success.

## Testing

Go conventions (`-race -count=1`, table-driven). Functional focus:

- **Config service:** valid write applies; invalid rejected with nothing written;
  backup created; atomic-rename path.
- **pipelock client:** parses `/stats` / `/sessions` / killswitch responses;
  degrades gracefully when pipelock is down.
- **Event sink + SSE:** ring buffer eviction; fan-out to multiple browser clients.
- **Service controller:** restart runs the correct command.
- **Frontend:** config-editor validation feedback + kill-switch confirm flow.

Explicitly out of scope: auth lockout / constant-time / cookie-flag / ingest-secret tests.

## Open Questions for Planning

- Config editor library (Monaco vs CodeMirror) and exact set of structured
  quick-toggles for v1.
- Console config file location and first-run password setup flow.
- Whether the console's web build is committed or built in CI for releases.
