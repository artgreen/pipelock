# Pipelock Console — Operator Guide

Pipelock Console is a standalone web application for operating a local pipelock
instance. It runs on the **same host** as pipelock and exposes a browser UI
reachable over the network — no SSH required. From the console you can:

- Watch the live event stream (DLP hits, injection blocks, kill-switch toggles)
- Inspect and edit `pipelock.yaml` with inline validation
- Toggle the API-sourced kill switch
- Restart the pipelock systemd unit

---

## Build and run

```bash
# Build both the frontend bundle and the Go binary
make console          # produces ./pipelock-console

# Start the server
pipelock-console serve --config /usr/local/etc/pipelock-console.yaml
```

`make console` runs `npm ci && npm run build` inside `internal/console/web/app/`
and then compiles the Go binary with the static assets embedded. The result is a
single self-contained binary with no runtime Node.js dependency.

> **Note:** GoReleaser is not yet wired to publish `pipelock-console` as a
> release artifact. Until that follow-up ships, build from source.

---

## Config file (`pipelock-console.yaml`)

The console reads its own YAML config — separate from `pipelock.yaml`.

```yaml
# Address and port to listen on. Default: 0.0.0.0:9443
listen: "0.0.0.0:9443"

# TLS — optional. When both fields are set the server serves HTTPS directly.
# If you terminate TLS in front (Traefik, Caddy, nginx), leave this blank.
tls:
  cert_file: ""   # e.g. /etc/pipelock-console/tls.crt
  key_file:  ""   # e.g. /etc/pipelock-console/tls.key

# How to reach the pipelock instance this console manages.
pipelock:
  base_url:       "http://127.0.0.1:8888"  # default; pipelock's listen address
  killswitch_url: ""                        # defaults to base_url; override if
                                            # kill_switch.api_listen is set in
                                            # pipelock.yaml (e.g. :9090)
  api_token:      ""                        # must match kill_switch.api_token

# Path to the pipelock.yaml managed by this console.
# Default: /usr/local/etc/pipelock.yaml
config_path: "/usr/local/etc/pipelock.yaml"

# systemd unit name for service status / restart.
# Default: pipelock
service_unit: "pipelock"

# Argon2id hash of the admin password.
# Leave blank on first install; the browser wizard sets it.
admin_password_hash: ""

# 64-hex-char session signing secret.
# Auto-generated and persisted on first startup — do not set by hand.
session_secret: ""
```

All fields have defaults (see the table above); a minimal working file is an
empty `{}` on first run.

---

## First-run: set the admin password

Leave `admin_password_hash` blank. On the first browser visit the console
displays a set-password wizard before showing any other UI. The wizard calls
`POST /api/setup` with the chosen password, hashes it with argon2id, and writes
the hash back to the config file.

Once a hash is set, `POST /api/setup` returns 409 — the wizard is disabled.

---

## Wiring the live event feed

The event stream (`/api/events`) is fed by pipelock POSTing audit events to the
console's `/ingest` endpoint. Configure pipelock's webhook emitter in
`pipelock.yaml`:

```yaml
emit:
  webhook:
    url: "http://127.0.0.1:9443/ingest"
    min_severity: info   # info | warn | critical
```

The `/ingest` endpoint is unauthenticated — it is only reachable on the local
host (bind `listen` to a loopback or private interface if the console host is
multi-tenant).

Additional webhook fields (all optional):

| Field             | Default | Description                         |
|-------------------|---------|-------------------------------------|
| `auth_token`      | —       | Bearer token attached to POSTs      |
| `timeout_seconds` | 5       | Per-request HTTP timeout            |
| `queue_size`      | 64      | Async delivery buffer size          |

---

## Kill switch wiring

The console's kill-switch toggle calls pipelock's `/api/v1/killswitch` API. For
this to work:

1. Set `kill_switch.api_token` in `pipelock.yaml` (or the env var
   `PIPELOCK_KILLSWITCH_API_TOKEN`).
2. Put the same token in the console config's `pipelock.api_token` field
   (or set `PIPELOCK_KILLSWITCH_API_TOKEN` in the console's environment — it
   overrides the config file value at runtime).
3. If pipelock runs the kill-switch API on a separate port via
   `kill_switch.api_listen` (e.g. `"0.0.0.0:9090"`), set
   `pipelock.killswitch_url` accordingly (e.g. `"http://127.0.0.1:9090"`).

---

## Service control

The console calls `systemctl is-active <service_unit>` and
`systemctl restart <service_unit>`. Run the console under a user or systemd unit
that is permitted to issue those commands against the pipelock unit — for
example:

```
# /etc/sudoers.d/pipelock-console
console-user ALL=(root) NOPASSWD: /bin/systemctl is-active pipelock
console-user ALL=(root) NOPASSWD: /bin/systemctl restart pipelock
```

Or grant the console's unit `ExecStartPost` permission via systemd's
`SupplementaryGroups` / `AmbientCapabilities` depending on your distro policy.
No SSH is needed or used.

---

## Security

- **Single admin password** gates all `/api/*` routes. `/ingest` is
  intentionally open so pipelock can POST events without authentication.
- **Put TLS in front** — either set `tls.cert_file` / `tls.key_file` for
  built-in HTTPS, or place a reverse proxy (Traefik, Caddy, nginx) in front.
  The console runs on the network; never expose it over plain HTTP to untrusted
  networks.
- The `session_secret` is auto-generated on first startup and stored in the
  config file (mode `0600`). Rotating it invalidates all active sessions.
