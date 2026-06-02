# Pipelock Console Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `pipelock-console`, a standalone web app (new binary in the pipelock repo) that lets an operator observe pipelock traffic, edit/validate config, flip the kill switch, and restart the service from a browser on the same host — no SSH.

**Architecture:** A Go `net/http` backend (`internal/console/*`, entry at `cmd/pipelock-console`) imports `internal/config` to reuse pipelock's real validation, proxies pipelock's existing HTTP API to the browser, receives pipelock's emitted webhook events into an in-memory ring buffer, and streams updates over SSE. A Vite + React + Tailwind + shadcn/ui frontend ("cyber-terminal" theme) is built to static assets and embedded via `go:embed`. Single admin password (argon2id) gates access via a first-run wizard.

**Tech Stack:** Go 1.25+ (stdlib `net/http`, `golang.org/x/crypto/argon2` — already a dep), CodeMirror 6, React, Tailwind, shadcn/ui, Vite, Server-Sent Events.

---

## Conventions (read once, apply to every task)

- **Module:** `github.com/luckyPipewrench/pipelock`. Console code lives under `internal/console/` (importable within the module) with the entry point at `cmd/pipelock-console/`.
- **File header** (top of every new `.go` file):
  ```go
  // Copyright 2026 Josh Waldrep
  // SPDX-License-Identifier: Apache-2.0
  ```
- **Lint/format:** run `gofumpt -w <file>` after editing; `golangci-lint run ./...` before tests. File perms `0o600`, dir perms `0o750`. Ignored errors as `_ = fn()`. Use `http.MethodGet` etc., not string literals.
- **Tests:** `go test -race -count=1 ./...`. Table-driven with `t.Run`. Test files end `_test.go`.
- **Commit cadence:** one commit per task (after its tests pass). Work on branch `feat/pipelock-console` (already created).

### Console config schema (referenced throughout)

`/usr/local/etc/pipelock-console.yaml`:
```yaml
listen: "0.0.0.0:9443"
tls:
  cert_file: ""          # if both set, serve HTTPS; else plain HTTP (expect Traefik in front)
  key_file: ""
pipelock:
  base_url: "http://127.0.0.1:8888"   # /health /stats /api/v1/sessions
  killswitch_url: ""                   # optional; defaults to base_url if blank
  api_token: ""                        # Bearer for kill switch API (or env PIPELOCK_KILLSWITCH_API_TOKEN)
config_path: "/usr/local/etc/pipelock.yaml"
service_unit: "pipelock"               # systemd unit
admin_password_hash: ""                # argon2id encoded; blank => first-run wizard
session_secret: ""                     # cookie signing key (hex); generated + persisted if blank
```

### File structure (created by this plan)

```
cmd/pipelock-console/main.go                 entry: cobra root + serve command
internal/console/config/config.go            ConsoleConfig type + Load + Save
internal/console/config/config_test.go
internal/console/pipelockclient/client.go    typed client for pipelock API
internal/console/pipelockclient/client_test.go
internal/console/configsvc/service.go        read / validate / atomic-write pipelock.yaml
internal/console/configsvc/service_test.go
internal/console/events/buffer.go            ring buffer of events
internal/console/events/buffer_test.go
internal/console/events/ingest.go            /ingest webhook receiver
internal/console/events/ingest_test.go
internal/console/events/sse.go               SSE hub fan-out
internal/console/events/sse_test.go
internal/console/auth/password.go            argon2id hash + verify
internal/console/auth/password_test.go
internal/console/auth/session.go             cookie session store + middleware
internal/console/auth/session_test.go
internal/console/service/controller.go       systemctl status/restart
internal/console/service/controller_test.go
internal/console/server/server.go            router wiring everything + static embed
internal/console/server/server_test.go
internal/console/web/embed.go                go:embed dist
internal/console/web/app/                     Vite React app (source)
internal/console/web/dist/                     build output (gitignored, embedded)
```

---

## Phase 0 — Scaffolding

### Task 1: Console config type, loader, and saver

**Files:**
- Create: `internal/console/config/config.go`
- Test: `internal/console/config/config_test.go`

- [ ] **Step 1: Write the failing test**

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsAndOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pipelock-console.yaml")
	yaml := "listen: \"127.0.0.1:9999\"\npipelock:\n  base_url: \"http://127.0.0.1:8888\"\nconfig_path: \"/tmp/pipelock.yaml\"\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen != "127.0.0.1:9999" {
		t.Errorf("Listen = %q", cfg.Listen)
	}
	if cfg.ServiceUnit != "pipelock" {
		t.Errorf("ServiceUnit default = %q, want pipelock", cfg.ServiceUnit)
	}
	if cfg.Pipelock.KillswitchURL != "http://127.0.0.1:8888" {
		t.Errorf("KillswitchURL should default to base_url, got %q", cfg.Pipelock.KillswitchURL)
	}
}

func TestLoadGeneratesSessionSecret(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(path, []byte("config_path: /tmp/p.yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.SessionSecret) < 32 {
		t.Errorf("expected generated session_secret, got %q", cfg.SessionSecret)
	}
	// Reload: secret must persist (be written back).
	cfg2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.SessionSecret != cfg.SessionSecret {
		t.Errorf("session_secret not persisted across loads")
	}
}

func TestSaveRoundTripsAdminPasswordHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(path, []byte("config_path: /tmp/p.yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.AdminPasswordHash = "$argon2id$v=19$..."
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.AdminPasswordHash != "$argon2id$v=19$..." {
		t.Errorf("hash not persisted: %q", reloaded.AdminPasswordHash)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/console/config/ -run TestLoad -v`
Expected: FAIL — package/`Load` undefined.

- [ ] **Step 3: Write the implementation**

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package config loads and persists the pipelock-console application config.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// TLS holds optional cert/key paths. When both are set the server serves HTTPS.
type TLS struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// Pipelock points the console at the pipelock instance it manages.
type Pipelock struct {
	BaseURL       string `yaml:"base_url"`
	KillswitchURL string `yaml:"killswitch_url"`
	APIToken      string `yaml:"api_token"`
}

// ConsoleConfig is the console's own configuration (distinct from pipelock.yaml).
type ConsoleConfig struct {
	Listen            string   `yaml:"listen"`
	TLS               TLS      `yaml:"tls"`
	Pipelock          Pipelock `yaml:"pipelock"`
	ConfigPath        string   `yaml:"config_path"`
	ServiceUnit       string   `yaml:"service_unit"`
	AdminPasswordHash string   `yaml:"admin_password_hash"`
	SessionSecret     string   `yaml:"session_secret"`
}

// Load reads the console config, applies defaults, generates and persists a
// session secret on first use, and returns the resolved config.
func Load(path string) (*ConsoleConfig, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("reading console config %s: %w", path, err)
	}
	cfg := &ConsoleConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing console config %s: %w", path, err)
	}

	if cfg.Listen == "" {
		cfg.Listen = "0.0.0.0:9443"
	}
	if cfg.ServiceUnit == "" {
		cfg.ServiceUnit = "pipelock"
	}
	if cfg.Pipelock.BaseURL == "" {
		cfg.Pipelock.BaseURL = "http://127.0.0.1:8888"
	}
	if cfg.Pipelock.KillswitchURL == "" {
		cfg.Pipelock.KillswitchURL = cfg.Pipelock.BaseURL
	}
	if cfg.ConfigPath == "" {
		cfg.ConfigPath = "/usr/local/etc/pipelock.yaml"
	}
	if token := os.Getenv("PIPELOCK_KILLSWITCH_API_TOKEN"); token != "" {
		cfg.Pipelock.APIToken = token
	}

	if cfg.SessionSecret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("generating session secret: %w", err)
		}
		cfg.SessionSecret = hex.EncodeToString(b)
		if err := Save(path, cfg); err != nil {
			return nil, fmt.Errorf("persisting session secret: %w", err)
		}
	}
	return cfg, nil
}

// Save writes the console config back to disk atomically (temp + rename).
func Save(path string, cfg *ConsoleConfig) error {
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling console config: %w", err)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".pipelock-console-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp config: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofumpt -w internal/console/config/ && go test ./internal/console/config/ -race -count=1 -v`
Expected: PASS (all three tests).

- [ ] **Step 5: Commit**

```bash
git add internal/console/config/
git commit -m "feat(console): console config load/save with defaults and session secret"
```

---

### Task 2: `cmd/pipelock-console` entry + `serve` command skeleton

**Files:**
- Create: `cmd/pipelock-console/main.go`
- Test: `cmd/pipelock-console/main_test.go`

- [ ] **Step 1: Write the failing test** (verifies the cobra root builds and `--help` works without booting a server)

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootHelp(t *testing.T) {
	cmd := newRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute --help: %v", err)
	}
	if !strings.Contains(buf.String(), "serve") {
		t.Errorf("help should mention the serve command, got:\n%s", buf.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/pipelock-console/ -run TestRootHelp -v`
Expected: FAIL — `newRootCmd` undefined.

- [ ] **Step 3: Write the implementation** (server boot wired in Task 17; here `serve` only resolves config path)

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package main is the entry point for the pipelock-console web app.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "pipelock-console",
		Short: "Web console for operating a local pipelock instance",
	}
	root.AddCommand(newServeCmd())
	return root
}

func newServeCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the web console",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Implemented in Task 17 (runServe).
			return runServe(cmd, configPath)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "/usr/local/etc/pipelock-console.yaml", "path to console config")
	return cmd
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

Add a temporary stub so the package compiles before Task 17 (delete the stub in Task 17):

```go
// runServe is replaced in Task 17. Temporary stub to allow compilation.
func runServe(_ *cobra.Command, _ string) error { return nil }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofumpt -w cmd/pipelock-console/ && go test ./cmd/pipelock-console/ -race -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/pipelock-console/
git commit -m "feat(console): cobra entry point and serve command skeleton"
```

---

## Phase 1 — Config service (validate-before-write, fail-closed)

### Task 3: Validate submitted YAML using pipelock's real validator

**Files:**
- Create: `internal/console/configsvc/service.go`
- Test: `internal/console/configsvc/service_test.go`

- [ ] **Step 1: Write the failing test**

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package configsvc

import (
	"strings"
	"testing"
)

func TestValidateAcceptsGoodConfig(t *testing.T) {
	good := "mode: block\nenforce: true\n"
	res := Validate([]byte(good))
	if !res.OK {
		t.Fatalf("expected valid, got error: %s", res.Error)
	}
}

func TestValidateRejectsUnknownField(t *testing.T) {
	bad := "mode: block\nbogus_field: true\n"
	res := Validate([]byte(bad))
	if res.OK {
		t.Fatal("expected invalid config to be rejected")
	}
	if !strings.Contains(strings.ToLower(res.Error), "bogus_field") {
		t.Errorf("error should name the offending field, got: %s", res.Error)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/console/configsvc/ -run TestValidate -v`
Expected: FAIL — `Validate` undefined.

- [ ] **Step 3: Write the implementation** (reuses `config.Load` semantics by parsing through the same strict decoder + `ValidateWithWarnings`)

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package configsvc reads, validates, and atomically writes the pipelock.yaml
// managed by the console. Validation reuses pipelock's real config package so
// it can never drift from the running proxy.
package configsvc

import (
	"bytes"
	"errors"
	"io"

	pcfg "github.com/luckyPipewrench/pipelock/internal/config"
	"gopkg.in/yaml.v3"
)

// ValidationResult reports whether submitted YAML is a valid pipelock config.
type ValidationResult struct {
	OK       bool     `json:"ok"`
	Error    string   `json:"error,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// Validate parses raw YAML with the same strict decoder pipelock uses
// (unknown fields rejected) and runs the real validator.
func Validate(raw []byte) ValidationResult {
	cfg := &pcfg.Config{}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
		return ValidationResult{OK: false, Error: err.Error()}
	}
	warns, err := cfg.ValidateWithWarnings()
	res := ValidationResult{OK: err == nil}
	if err != nil {
		res.Error = err.Error()
	}
	for _, w := range warns {
		res.Warnings = append(res.Warnings, w.Field+": "+w.Message)
	}
	return res
}
```

> Confirmed: `pcfg.Warning` is `struct{ Field, Message string }` (internal/config/validate.go:63) — no `String()` method, so format `Field + ": " + Message` as above.

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofumpt -w internal/console/configsvc/ && go test ./internal/console/configsvc/ -race -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/console/configsvc/
git commit -m "feat(console): validate submitted config via pipelock's real validator"
```

---

### Task 4: Read the current pipelock.yaml

**Files:**
- Modify: `internal/console/configsvc/service.go`
- Test: `internal/console/configsvc/service_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestServiceReadsCurrentConfig(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/pipelock.yaml"
	if err := os.WriteFile(path, []byte("mode: block\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := New(path)
	got, err := svc.Read()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "mode: block\n" {
		t.Errorf("Read() = %q", got)
	}
}
```

Add imports `os` and `testing` (already present) at the top of the test file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/console/configsvc/ -run TestServiceReads -v`
Expected: FAIL — `New`/`Read` undefined.

- [ ] **Step 3: Write the implementation** (append to `service.go`)

```go
import (
	// add to existing import block:
	"fmt"
	"os"
	"path/filepath"
)

// Service manages the on-disk pipelock config at Path.
type Service struct {
	Path string
}

// New returns a Service for the pipelock config at path.
func New(path string) *Service { return &Service{Path: path} }

// Read returns the current pipelock.yaml contents.
func (s *Service) Read() ([]byte, error) {
	data, err := os.ReadFile(filepath.Clean(s.Path))
	if err != nil {
		return nil, fmt.Errorf("reading pipelock config: %w", err)
	}
	return data, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofumpt -w internal/console/configsvc/ && go test ./internal/console/configsvc/ -race -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/console/configsvc/
git commit -m "feat(console): read current pipelock config"
```

---

### Task 5: Validate-then-atomic-write with timestamped backup (fail-closed)

**Files:**
- Modify: `internal/console/configsvc/service.go`
- Test: `internal/console/configsvc/service_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestWriteRejectsInvalidAndChangesNothing(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/pipelock.yaml"
	original := "mode: block\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := New(path)
	err := svc.Write([]byte("mode: block\nbogus_field: 1\n"))
	if err == nil {
		t.Fatal("expected write of invalid config to be rejected")
	}
	got, _ := os.ReadFile(path)
	if string(got) != original {
		t.Errorf("file mutated on rejected write: %q", got)
	}
}

func TestWriteAppliesValidConfigWithBackup(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/pipelock.yaml"
	if err := os.WriteFile(path, []byte("mode: block\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := New(path)
	if err := svc.Write([]byte("mode: warn\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "mode: warn\n" {
		t.Errorf("new config not written: %q", got)
	}
	backups, _ := filepath.Glob(path + ".bak.*")
	if len(backups) != 1 {
		t.Errorf("expected exactly one backup, got %d", len(backups))
	}
	b, _ := os.ReadFile(backups[0])
	if string(b) != "mode: block\n" {
		t.Errorf("backup should hold prior contents, got %q", b)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/console/configsvc/ -run TestWrite -v`
Expected: FAIL — `Write` undefined.

- [ ] **Step 3: Write the implementation** (append to `service.go`; `nowFunc` is injectable so backup names are deterministic in tests)

```go
import (
	// add:
	"time"
)

// nowFunc is overridable in tests for deterministic backup names.
var nowFunc = time.Now

// Write validates raw config and, only if valid, backs up the current file and
// atomically replaces it. Invalid input is rejected and nothing is written.
func (s *Service) Write(raw []byte) error {
	if res := Validate(raw); !res.OK {
		return fmt.Errorf("config rejected: %s", res.Error)
	}
	current, err := os.ReadFile(filepath.Clean(s.Path))
	if err != nil {
		return fmt.Errorf("reading current config for backup: %w", err)
	}
	backup := fmt.Sprintf("%s.bak.%s", s.Path, nowFunc().UTC().Format("20060102T150405Z"))
	if err := os.WriteFile(backup, current, 0o600); err != nil {
		return fmt.Errorf("writing backup: %w", err)
	}

	dir := filepath.Dir(s.Path)
	tmp, err := os.CreateTemp(dir, ".pipelock-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp config: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.Path)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofumpt -w internal/console/configsvc/ && go test ./internal/console/configsvc/ -race -count=1 -v`
Expected: PASS (both new tests + earlier ones).

- [ ] **Step 5: Commit**

```bash
git add internal/console/configsvc/
git commit -m "feat(console): validate-then-atomic-write pipelock config with backup"
```

---

## Phase 2 — pipelock client

### Task 6: Client `GetStats`

**Files:**
- Create: `internal/console/pipelockclient/client.go`
- Test: `internal/console/pipelockclient/client_test.go`

- [ ] **Step 1: Write the failing test** (uses `httptest` to fake pipelock)

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package pipelockclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stats" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"uptime_seconds":123,"requests":{"total":10,"allowed":7,"blocked":3,"block_rate":0.3},"sessions":{"active":2}}`))
	}))
	defer srv.Close()

	c := New(Options{BaseURL: srv.URL})
	stats, err := c.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.Requests.Blocked != 3 || stats.Sessions.Active != 2 {
		t.Errorf("unexpected stats: %+v", stats)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/console/pipelockclient/ -run TestGetStats -v`
Expected: FAIL — `New`/`GetStats` undefined.

- [ ] **Step 3: Write the implementation**

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package pipelockclient is a typed HTTP client for pipelock's runtime API.
package pipelockclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Options configures a Client.
type Options struct {
	BaseURL       string
	KillswitchURL string
	APIToken      string
	HTTP          *http.Client
}

// Client talks to a pipelock instance's HTTP API.
type Client struct {
	baseURL       string
	killswitchURL string
	apiToken      string
	http          *http.Client
}

// New constructs a Client, applying sensible defaults.
func New(o Options) *Client {
	if o.HTTP == nil {
		o.HTTP = &http.Client{Timeout: 5 * time.Second}
	}
	if o.KillswitchURL == "" {
		o.KillswitchURL = o.BaseURL
	}
	return &Client{baseURL: o.BaseURL, killswitchURL: o.KillswitchURL, apiToken: o.APIToken, http: o.HTTP}
}

// Stats mirrors pipelock's /stats JSON (subset the console renders).
type Stats struct {
	UptimeSeconds float64 `json:"uptime_seconds"`
	Requests      struct {
		Total     int64   `json:"total"`
		Allowed   int64   `json:"allowed"`
		Blocked   int64   `json:"blocked"`
		BlockRate float64 `json:"block_rate"`
	} `json:"requests"`
	Tunnels    int64 `json:"tunnels"`
	WebSockets int64 `json:"websockets"`
	TopBlockedDomains []struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	} `json:"top_blocked_domains"`
	TopScanners []struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	} `json:"top_scanners"`
	Sessions struct {
		Active      int64 `json:"active"`
		Anomalies   int64 `json:"anomalies"`
		Escalations int64 `json:"escalations"`
	} `json:"sessions"`
}

func (c *Client) getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	// /api/v1/sessions requires auth; /stats and /health ignore the header,
	// so attaching the token unconditionally is safe.
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// GetStats fetches /stats.
func (c *Client) GetStats(ctx context.Context) (*Stats, error) {
	var s Stats
	if err := c.getJSON(ctx, c.baseURL+"/stats", &s); err != nil {
		return nil, err
	}
	return &s, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofumpt -w internal/console/pipelockclient/ && go test ./internal/console/pipelockclient/ -race -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/console/pipelockclient/
git commit -m "feat(console): pipelock client GetStats"
```

---

### Task 7: Client `GetSessions` and `GetHealth`

**Files:**
- Modify: `internal/console/pipelockclient/client.go`
- Test: `internal/console/pipelockclient/client_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestGetSessions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"sessions":[{"key":"s1"},{"key":"s2"}],"count":2}`))
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL})
	got, err := c.GetSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Count != 2 || len(got.Sessions) != 2 {
		t.Errorf("unexpected sessions: %+v", got)
	}
}

func TestGetHealthReportsDownWhenUnreachable(t *testing.T) {
	c := New(Options{BaseURL: "http://127.0.0.1:1"}) // nothing listening
	if c.Healthy(context.Background()) {
		t.Error("expected Healthy=false for unreachable pipelock")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/console/pipelockclient/ -run 'TestGetSessions|TestGetHealth' -v`
Expected: FAIL — `GetSessions`/`Healthy` undefined.

- [ ] **Step 3: Write the implementation** (append; `Sessions.Sessions` is `json.RawMessage` so the console forwards pipelock's session shape verbatim to the browser without re-modeling every field)

```go
import (
	// add:
	"encoding/json"
)

// Sessions mirrors pipelock's /api/v1/sessions response.
type Sessions struct {
	Sessions []json.RawMessage `json:"sessions"`
	Count    int               `json:"count"`
}

// GetSessions fetches /api/v1/sessions.
func (c *Client) GetSessions(ctx context.Context) (*Sessions, error) {
	var s Sessions
	if err := c.getJSON(ctx, c.baseURL+"/api/v1/sessions", &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Healthy reports whether pipelock's /health endpoint is reachable and 200.
func (c *Client) Healthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK
}
```

(Remove the duplicate `encoding/json` import if Step 3 of Task 6 already imported it — keep one.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofumpt -w internal/console/pipelockclient/ && go test ./internal/console/pipelockclient/ -race -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/console/pipelockclient/
git commit -m "feat(console): pipelock client GetSessions and health check"
```

---

### Task 8: Client kill-switch status + toggle (Bearer auth)

**Files:**
- Modify: `internal/console/pipelockclient/client.go`
- Test: `internal/console/pipelockclient/client_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestKillSwitchToggleSendsBearer(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{"active":true,"source":"api"}`))
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL, APIToken: "secret123"})
	if err := c.SetKillSwitch(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer secret123" {
		t.Errorf("missing/incorrect bearer: %q", gotAuth)
	}
	if !strings.Contains(gotBody, `"active":true`) {
		t.Errorf("unexpected body: %q", gotBody)
	}
}
```

Add `strings` to the test imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/console/pipelockclient/ -run TestKillSwitch -v`
Expected: FAIL — `SetKillSwitch` undefined.

- [ ] **Step 3: Write the implementation** (append; uses `killswitchURL` + Bearer token; status via GET `/api/v1/killswitch/status`)

```go
import (
	// add:
	"bytes"
)

// KillSwitchStatus mirrors pipelock's /api/v1/killswitch/status response.
type KillSwitchStatus struct {
	Active  bool            `json:"active"`
	Sources map[string]bool `json:"sources"`
}

// GetKillSwitch fetches current kill switch status.
func (c *Client) GetKillSwitch(ctx context.Context) (*KillSwitchStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.killswitchURL+"/api/v1/killswitch/status", nil)
	if err != nil {
		return nil, err
	}
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("killswitch status: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("killswitch status returned %d", resp.StatusCode)
	}
	var s KillSwitchStatus
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

// SetKillSwitch toggles the API-sourced kill switch.
func (c *Client) SetKillSwitch(ctx context.Context, active bool) error {
	body, _ := json.Marshal(map[string]bool{"active": active})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.killswitchURL+"/api/v1/killswitch", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("killswitch toggle: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("killswitch toggle returned %d", resp.StatusCode)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofumpt -w internal/console/pipelockclient/ && go test ./internal/console/pipelockclient/ -race -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/console/pipelockclient/
git commit -m "feat(console): pipelock client kill switch status and toggle"
```

---

## Phase 3 — Event sink + SSE

### Task 9: Event ring buffer

**Files:**
- Create: `internal/console/events/buffer.go`
- Test: `internal/console/events/buffer_test.go`

- [ ] **Step 1: Write the failing test**

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package events

import "testing"

func TestRingBufferEvictsOldest(t *testing.T) {
	b := NewBuffer(3)
	for i := 1; i <= 5; i++ {
		b.Add(Event{Type: "t", Fields: map[string]any{"i": i}})
	}
	got := b.Snapshot()
	if len(got) != 3 {
		t.Fatalf("expected 3 retained, got %d", len(got))
	}
	if got[0].Fields["i"] != 3 || got[2].Fields["i"] != 5 {
		t.Errorf("expected oldest evicted, got %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/console/events/ -run TestRingBuffer -v`
Expected: FAIL — `NewBuffer`/`Event` undefined.

- [ ] **Step 3: Write the implementation**

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package events ingests pipelock's emitted events and fans them out to
// browser clients over SSE.
package events

import "sync"

// Event mirrors pipelock's webhook payload (internal/emit/webhook.go).
type Event struct {
	Severity  string         `json:"severity"`
	Type      string         `json:"type"`
	Timestamp string         `json:"timestamp"`
	Instance  string         `json:"pipelock_instance"`
	Fields    map[string]any `json:"fields"`
}

// Buffer is a fixed-capacity ring of recent events, safe for concurrent use.
type Buffer struct {
	mu    sync.Mutex
	items []Event
	cap   int
}

// NewBuffer creates a ring buffer retaining the most recent capacity events.
func NewBuffer(capacity int) *Buffer {
	if capacity < 1 {
		capacity = 1
	}
	return &Buffer{cap: capacity}
}

// Add appends an event, evicting the oldest when at capacity.
func (b *Buffer) Add(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.items = append(b.items, e)
	if len(b.items) > b.cap {
		b.items = b.items[len(b.items)-b.cap:]
	}
}

// Snapshot returns a copy of retained events, oldest first.
func (b *Buffer) Snapshot() []Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Event, len(b.items))
	copy(out, b.items)
	return out
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofumpt -w internal/console/events/ && go test ./internal/console/events/ -race -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/console/events/
git commit -m "feat(console): event ring buffer"
```

---

### Task 10: SSE hub (fan-out to browser clients)

**Files:**
- Create: `internal/console/events/sse.go`
- Test: `internal/console/events/sse_test.go`

- [ ] **Step 1: Write the failing test**

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"testing"
	"time"
)

func TestHubBroadcastsToSubscribers(t *testing.T) {
	h := NewHub()
	ch1 := h.Subscribe()
	ch2 := h.Subscribe()
	h.Broadcast(Event{Type: "ping"})

	for _, ch := range []<-chan Event{ch1, ch2} {
		select {
		case e := <-ch:
			if e.Type != "ping" {
				t.Errorf("got %q", e.Type)
			}
		case <-time.After(time.Second):
			t.Fatal("subscriber did not receive broadcast")
		}
	}
}

func TestHubDropsSlowSubscriberWithoutBlocking(t *testing.T) {
	h := NewHub()
	_ = h.Subscribe() // never drained
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			h.Broadcast(Event{Type: "x"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Broadcast blocked on a slow subscriber")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/console/events/ -run TestHub -v`
Expected: FAIL — `NewHub` undefined.

- [ ] **Step 3: Write the implementation** (non-blocking send; drop on full buffer so one stalled browser can't wedge the hub)

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package events

import "sync"

// Hub fans out events to subscribed SSE clients.
type Hub struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

// NewHub creates an empty Hub.
func NewHub() *Hub { return &Hub{subs: make(map[chan Event]struct{})} }

// Subscribe registers a new client channel (buffered).
func (h *Hub) Subscribe() <-chan Event {
	ch := make(chan Event, 64)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes and closes a client channel.
func (h *Hub) Unsubscribe(ch <-chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.subs {
		if c == ch {
			delete(h.subs, c)
			close(c)
			return
		}
	}
}

// Broadcast sends to all subscribers, dropping for any that are full.
func (h *Hub) Broadcast(e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.subs {
		select {
		case c <- e:
		default: // slow consumer — drop rather than block
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofumpt -w internal/console/events/ && go test ./internal/console/events/ -race -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/console/events/
git commit -m "feat(console): SSE hub with non-blocking fan-out"
```

---

### Task 11: `/ingest` webhook receiver

**Files:**
- Create: `internal/console/events/ingest.go`
- Test: `internal/console/events/ingest_test.go`

- [ ] **Step 1: Write the failing test**

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIngestStoresAndBroadcasts(t *testing.T) {
	buf := NewBuffer(10)
	hub := NewHub()
	ch := hub.Subscribe()
	h := IngestHandler(buf, hub)

	body := `{"severity":"critical","type":"dlp.secret","fields":{"target":"api.openai.com"}}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := buf.Snapshot(); len(got) != 1 || got[0].Type != "dlp.secret" {
		t.Errorf("event not buffered: %+v", got)
	}
	select {
	case e := <-ch:
		if e.Type != "dlp.secret" {
			t.Errorf("broadcast type = %q", e.Type)
		}
	default:
		t.Error("event not broadcast")
	}
}

func TestIngestRejectsNonPOST(t *testing.T) {
	h := IngestHandler(NewBuffer(1), NewHub())
	req := httptest.NewRequest(http.MethodGet, "/ingest", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/console/events/ -run TestIngest -v`
Expected: FAIL — `IngestHandler` undefined.

- [ ] **Step 3: Write the implementation**

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"encoding/json"
	"net/http"
)

const maxIngestBody = 64 * 1024

// IngestHandler returns an http.Handler that accepts pipelock webhook events,
// stores them in buf, and broadcasts them to hub subscribers.
func IngestHandler(buf *Buffer, hub *Hub) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxIngestBody)
		var e Event
		if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
			http.Error(w, "invalid event", http.StatusBadRequest)
			return
		}
		buf.Add(e)
		hub.Broadcast(e)
		w.WriteHeader(http.StatusNoContent)
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofumpt -w internal/console/events/ && go test ./internal/console/events/ -race -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/console/events/
git commit -m "feat(console): /ingest webhook receiver"
```

---

## Phase 4 — Auth (single password, first-run wizard)

### Task 12: Argon2id password hash + verify

**Files:**
- Create: `internal/console/auth/password.go`
- Test: `internal/console/auth/password_test.go`

- [ ] **Step 1: Write the failing test**

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	h, err := HashPassword("hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$") {
		t.Errorf("unexpected hash format: %q", h)
	}
	if !VerifyPassword(h, "hunter2") {
		t.Error("correct password should verify")
	}
	if VerifyPassword(h, "wrong") {
		t.Error("wrong password should not verify")
	}
}

func TestVerifyRejectsMalformedHash(t *testing.T) {
	if VerifyPassword("not-a-hash", "x") {
		t.Error("malformed hash must not verify")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/console/auth/ -run 'TestHash|TestVerify' -v`
Expected: FAIL — `HashPassword`/`VerifyPassword` undefined.

- [ ] **Step 3: Write the implementation** (encoded format `$argon2id$v=19$m=,t=,p=$salt$hash`; `golang.org/x/crypto` is already a pipelock dep)

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package auth provides single-password authentication for the console.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

// HashPassword returns an encoded argon2id hash of the password.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether password matches the encoded argon2id hash.
func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofumpt -w internal/console/auth/ && go test ./internal/console/auth/ -race -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/console/auth/
git commit -m "feat(console): argon2id password hashing"
```

---

### Task 13: Session cookie store + auth middleware + first-run state

**Files:**
- Create: `internal/console/auth/session.go`
- Test: `internal/console/auth/session_test.go`

- [ ] **Step 1: Write the failing test**

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestManager(t *testing.T, hash string) *Manager {
	t.Helper()
	return NewManager(Options{PasswordHash: hash, SecretHex: "00112233445566778899aabbccddeeff"})
}

func TestMiddlewareBlocksUnauthenticated(t *testing.T) {
	m := newTestManager(t, "$argon2id$dummy")
	protected := m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLoginThenAccess(t *testing.T) {
	hash, _ := HashPassword("pw")
	m := newTestManager(t, hash)

	// Login issues a cookie.
	loginRec := httptest.NewRecorder()
	if !m.Login(loginRec, "pw") {
		t.Fatal("login with correct password failed")
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie issued")
	}

	// Authenticated request passes.
	protected := m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookies[0])
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("authenticated request status = %d", rec.Code)
	}
}

func TestNeedsSetupWhenNoPasswordHash(t *testing.T) {
	m := newTestManager(t, "")
	if !m.NeedsSetup() {
		t.Error("expected NeedsSetup=true with empty hash")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/console/auth/ -run 'TestMiddleware|TestLogin|TestNeedsSetup' -v`
Expected: FAIL — `NewManager` undefined.

- [ ] **Step 3: Write the implementation** (HMAC-signed cookie token; in-memory session set; `NeedsSetup` drives the first-run wizard)

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sync"
)

const cookieName = "pipelock_console_session"

// Options configures a Manager.
type Options struct {
	PasswordHash string
	SecretHex    string
}

// Manager handles login, session cookies, and auth middleware.
type Manager struct {
	passwordHash string
	secret       []byte
	mu           sync.Mutex
	sessions     map[string]struct{}
}

// NewManager constructs a Manager.
func NewManager(o Options) *Manager {
	secret, _ := hex.DecodeString(o.SecretHex)
	return &Manager{passwordHash: o.PasswordHash, secret: secret, sessions: make(map[string]struct{})}
}

// NeedsSetup reports whether no admin password has been set yet.
func (m *Manager) NeedsSetup() bool { return m.passwordHash == "" }

// SetPasswordHash updates the active password hash (used by first-run wizard).
func (m *Manager) SetPasswordHash(hash string) { m.passwordHash = hash }

func (m *Manager) sign(token string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

// Login verifies the password and, on success, sets a session cookie.
func (m *Manager) Login(w http.ResponseWriter, password string) bool {
	if m.passwordHash == "" || !VerifyPassword(m.passwordHash, password) {
		return false
	}
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	token := hex.EncodeToString(raw)
	m.mu.Lock()
	m.sessions[token] = struct{}{}
	m.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token + "." + m.sign(token),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	return true
}

// Logout invalidates the request's session.
func (m *Manager) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		if token, _, ok := splitToken(c.Value); ok {
			m.mu.Lock()
			delete(m.sessions, token)
			m.mu.Unlock()
		}
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1})
}

func splitToken(v string) (token, sig string, ok bool) {
	for i := 0; i < len(v); i++ {
		if v[i] == '.' {
			return v[:i], v[i+1:], true
		}
	}
	return "", "", false
}

func (m *Manager) valid(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	token, sig, ok := splitToken(c.Value)
	if !ok || !hmac.Equal([]byte(sig), []byte(m.sign(token))) {
		return false
	}
	m.mu.Lock()
	_, exists := m.sessions[token]
	m.mu.Unlock()
	return exists
}

// RequireAuth wraps a handler, returning 401 for unauthenticated requests.
func (m *Manager) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.valid(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofumpt -w internal/console/auth/ && go test ./internal/console/auth/ -race -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/console/auth/
git commit -m "feat(console): session cookies, auth middleware, first-run state"
```

---

## Phase 5 — Service control

### Task 14: systemctl status/restart controller (injection-safe)

**Files:**
- Create: `internal/console/service/controller.go`
- Test: `internal/console/service/controller_test.go`

- [ ] **Step 1: Write the failing test** (inject a fake runner so no real systemd is needed; verify exact argv)

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"reflect"
	"testing"
)

func TestRestartInvokesExactCommand(t *testing.T) {
	var gotArgs []string
	c := &Controller{Unit: "pipelock", run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		gotArgs = append([]string{name}, args...)
		return []byte("ok"), nil
	}}
	if _, err := c.Restart(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"systemctl", "restart", "pipelock"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Errorf("argv = %v, want %v", gotArgs, want)
	}
}

func TestStatusInvokesExactCommand(t *testing.T) {
	var gotArgs []string
	c := &Controller{Unit: "pipelock", run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		gotArgs = append([]string{name}, args...)
		return []byte("active"), nil
	}}
	out, err := c.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"systemctl", "is-active", "pipelock"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Errorf("argv = %v, want %v", gotArgs, want)
	}
	if out != "active" {
		t.Errorf("status = %q", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/console/service/ -run 'TestRestart|TestStatus' -v`
Expected: FAIL — `Controller` undefined.

- [ ] **Step 3: Write the implementation** (the unit name is config-provided, never request input; argv is fixed so there's no shell to inject into)

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package service controls the local pipelock systemd unit.
package service

import (
	"context"
	"os/exec"
	"strings"
)

type runner func(ctx context.Context, name string, args ...string) ([]byte, error)

func execRun(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// Controller runs systemctl actions against a fixed unit.
type Controller struct {
	Unit string
	run  runner
}

// New returns a Controller for the given unit using the real systemctl.
func New(unit string) *Controller {
	return &Controller{Unit: unit, run: execRun}
}

// Status returns `systemctl is-active <unit>` output (e.g. "active").
func (c *Controller) Status(ctx context.Context) (string, error) {
	out, err := c.run(ctx, "systemctl", "is-active", c.Unit)
	return strings.TrimSpace(string(out)), err
}

// Restart runs `systemctl restart <unit>`.
func (c *Controller) Restart(ctx context.Context) (string, error) {
	out, err := c.run(ctx, "systemctl", "restart", c.Unit)
	return strings.TrimSpace(string(out)), err
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `gofumpt -w internal/console/service/ && go test ./internal/console/service/ -race -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/console/service/
git commit -m "feat(console): systemctl status/restart controller"
```

---

## Phase 6 — HTTP wiring + embed

### Task 15: Web asset embed package (with placeholder until frontend exists)

**Files:**
- Create: `internal/console/web/embed.go`
- Create: `internal/console/web/dist/index.html` (placeholder; real build overwrites in Task 18)
- Modify: `.gitignore` (ignore built dist except the placeholder is committed)

- [ ] **Step 1: Create the placeholder dist so `go:embed` compiles**

`internal/console/web/dist/index.html`:
```html
<!doctype html><html><head><meta charset="utf-8"><title>pipelock-console</title></head>
<body><div id="root">console UI build not present — run `make console-web`</div></body></html>
```

- [ ] **Step 2: Write the embed package**

`internal/console/web/embed.go`:
```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package web embeds the built console frontend.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS returns the embedded built frontend rooted at dist/.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist is embedded at build time; absence is a build error
	}
	return sub
}
```

- [ ] **Step 3: Add a test that the embed is non-empty**

`internal/console/web/embed_test.go`:
```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package web

import (
	"io/fs"
	"testing"
)

func TestEmbedHasIndex(t *testing.T) {
	if _, err := fs.Stat(FS(), "index.html"); err != nil {
		t.Fatalf("embedded dist missing index.html: %v", err)
	}
}
```

- [ ] **Step 4: Run + commit**

Run: `go test ./internal/console/web/ -race -count=1 -v`
Expected: PASS.

```bash
git add internal/console/web/
git commit -m "feat(console): embed built frontend assets (placeholder)"
```

---

### Task 16: HTTP API router (REST + SSE + static) under auth

**Files:**
- Create: `internal/console/server/server.go`
- Test: `internal/console/server/server_test.go`

This task wires the pieces into one `http.Handler`. Routes:

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/api/setup` | none | `{needs_setup: bool}` for the wizard |
| POST | `/api/setup` | none (only when NeedsSetup) | set first admin password |
| POST | `/api/login` | none | login, set cookie |
| POST | `/api/logout` | yes | clear session |
| GET | `/api/stats` | yes | proxy pipelock `/stats` |
| GET | `/api/sessions` | yes | proxy pipelock sessions |
| GET | `/api/killswitch` | yes | killswitch status |
| POST | `/api/killswitch` | yes | toggle killswitch |
| GET | `/api/config` | yes | read pipelock.yaml |
| POST | `/api/config/validate` | yes | validate (no write) |
| POST | `/api/config` | yes | validate + write |
| GET | `/api/service` | yes | systemctl is-active |
| POST | `/api/service/restart` | yes | restart |
| POST | `/ingest` | none (localhost) | pipelock webhook in |
| GET | `/api/events` | yes | SSE stream |
| GET | `/*` | none | static frontend (SPA fallback to index.html) |

- [ ] **Step 1: Write the failing test** (covers auth gating, config write path, and SSE headers)

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/luckyPipewrench/pipelock/internal/console/auth"
	"github.com/luckyPipewrench/pipelock/internal/console/configsvc"
	"github.com/luckyPipewrench/pipelock/internal/console/events"
	"github.com/luckyPipewrench/pipelock/internal/console/pipelockclient"
	"github.com/luckyPipewrench/pipelock/internal/console/service"
)

func newTestServer(t *testing.T, configPath, passwordHash string) http.Handler {
	t.Helper()
	mgr := auth.NewManager(auth.Options{PasswordHash: passwordHash, SecretHex: "00112233445566778899aabbccddeeff"})
	return New(Deps{
		Auth:    mgr,
		Config:  configsvc.New(configPath),
		Client:  pipelockclient.New(pipelockclient.Options{BaseURL: "http://127.0.0.1:1"}),
		Service: &service.Controller{Unit: "pipelock"},
		Buffer:  events.NewBuffer(100),
		Hub:     events.NewHub(),
	})
}

func TestConfigEndpointRequiresAuth(t *testing.T) {
	h := newTestServer(t, "/tmp/none.yaml", "$argon2id$x")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestSetupReportsNeedsSetup(t *testing.T) {
	h := newTestServer(t, "/tmp/none.yaml", "")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/setup", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "true") {
		t.Errorf("setup status: code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestConfigWriteAppliesValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/pipelock.yaml"
	_ = os.WriteFile(path, []byte("mode: block\n"), 0o600)
	hash, _ := auth.HashPassword("pw")
	h := newTestServer(t, path, hash)

	// login
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"password":"pw"}`)))
	cookie := loginRec.Result().Cookies()[0]

	// write
	req := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader("mode: warn\n"))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("write status = %d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := os.ReadFile(path)
	if string(got) != "mode: warn\n" {
		t.Errorf("config not written: %q", got)
	}
	_ = context.Background()
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/console/server/ -run 'TestConfig|TestSetup' -v`
Expected: FAIL — `New`/`Deps` undefined.

- [ ] **Step 3: Write the implementation** (`server.go`). Build a `*http.ServeMux`; wrap protected routes with `Auth.RequireAuth`; serve static via `http.FileServerFS(web.FS())` with SPA fallback. Handlers call the components built in earlier tasks.

```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package server wires console components into one HTTP handler.
package server

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/luckyPipewrench/pipelock/internal/console/auth"
	"github.com/luckyPipewrench/pipelock/internal/console/configsvc"
	"github.com/luckyPipewrench/pipelock/internal/console/events"
	"github.com/luckyPipewrench/pipelock/internal/console/pipelockclient"
	"github.com/luckyPipewrench/pipelock/internal/console/service"
	"github.com/luckyPipewrench/pipelock/internal/console/web"
)

// Deps holds the wired-in console components.
type Deps struct {
	Auth     *auth.Manager
	Config   *configsvc.Service
	Client   *pipelockclient.Client
	Service  *service.Controller
	Buffer   *events.Buffer
	Hub      *events.Hub
	OnPasswordSet func(hash string) // persists hash to console config (set in Task 17)
}

// New builds the console HTTP handler.
func New(d Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/setup", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]bool{"needs_setup": d.Auth.NeedsSetup()})
	})
	mux.HandleFunc("POST /api/setup", func(w http.ResponseWriter, r *http.Request) {
		if !d.Auth.NeedsSetup() {
			http.Error(w, "already configured", http.StatusConflict)
			return
		}
		pw := decodePassword(w, r)
		if pw == "" {
			return
		}
		hash, err := auth.HashPassword(pw)
		if err != nil {
			http.Error(w, "hash error", http.StatusInternalServerError)
			return
		}
		d.Auth.SetPasswordHash(hash)
		if d.OnPasswordSet != nil {
			d.OnPasswordSet(hash)
		}
		d.Auth.Login(w, pw)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		pw := decodePassword(w, r)
		if pw == "" {
			return
		}
		if !d.Auth.Login(w, pw) {
			http.Error(w, "invalid password", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.Handle("POST /api/logout", d.Auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d.Auth.Logout(w, r)
		w.WriteHeader(http.StatusNoContent)
	})))
	mux.Handle("GET /api/stats", d.Auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, err := d.Client.GetStats(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, s)
	})))
	mux.Handle("GET /api/sessions", d.Auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, err := d.Client.GetSessions(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, s)
	})))
	mux.Handle("GET /api/killswitch", d.Auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ks, err := d.Client.GetKillSwitch(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, ks)
	})))
	mux.Handle("POST /api/killswitch", d.Auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Active bool `json:"active"` }
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := d.Client.SetKillSwitch(r.Context(), body.Active); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	mux.Handle("GET /api/config", d.Auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		raw, err := d.Config.Read()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/yaml")
		_, _ = w.Write(raw)
	})))
	mux.Handle("POST /api/config/validate", d.Auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		writeJSON(w, configsvc.Validate(raw))
	})))
	mux.Handle("POST /api/config", d.Auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err := d.Config.Write(raw); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})))
	mux.Handle("GET /api/service", d.Auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		st, _ := d.Service.Status(r.Context())
		writeJSON(w, map[string]string{"status": st})
	})))
	mux.Handle("POST /api/service/restart", d.Auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out, err := d.Service.Restart(r.Context())
		if err != nil {
			http.Error(w, out, http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"output": out})
	})))
	mux.Handle("GET /api/events", d.Auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveSSE(w, r, d.Hub, d.Buffer)
	})))

	mux.Handle("POST /ingest", events.IngestHandler(d.Buffer, d.Hub))

	// Static SPA with fallback to index.html for client-side routes.
	fileServer := http.FileServerFS(web.FS())
	mux.Handle("GET /", spaFallback(fileServer))

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func decodePassword(w http.ResponseWriter, r *http.Request) string {
	var body struct{ Password string `json:"password"` }
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil || body.Password == "" {
		http.Error(w, "password required", http.StatusBadRequest)
		return ""
	}
	return body.Password
}
```

- [ ] **Step 4: Add `serveSSE` and `spaFallback` helpers**

`internal/console/server/sse.go`:
```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"net/http"

	"github.com/luckyPipewrench/pipelock/internal/console/events"
)

func serveSSE(w http.ResponseWriter, r *http.Request, hub *events.Hub, buf *events.Buffer) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	// Replay recent buffer first so a fresh page isn't empty.
	for _, e := range buf.Snapshot() {
		writeSSE(w, e)
	}
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(w, e)
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, e events.Event) {
	data, _ := json.Marshal(e)
	_, _ = w.Write([]byte("data: "))
	_, _ = w.Write(data)
	_, _ = w.Write([]byte("\n\n"))
}
```

`internal/console/server/spa.go`:
```go
// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"
	"strings"

	"github.com/luckyPipewrench/pipelock/internal/console/web"
)

// spaFallback serves static files, falling back to index.html for non-asset
// paths so client-side routing works on deep links.
func spaFallback(fileServer http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := web.FS().Open(strings.TrimPrefix(r.URL.Path, "/")); err != nil && r.URL.Path != "/" {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 5: Run tests, lint, commit**

Run: `gofumpt -w internal/console/server/ && golangci-lint run ./internal/console/... && go test ./internal/console/server/ -race -count=1 -v`
Expected: PASS.

```bash
git add internal/console/server/
git commit -m "feat(console): HTTP router with REST, SSE, static SPA, and auth"
```

---

### Task 17: Wire `serve` to boot the real server

**Files:**
- Modify: `cmd/pipelock-console/main.go` (replace the `runServe` stub)
- Test: `cmd/pipelock-console/main_test.go`

- [ ] **Step 1: Write the failing test** (boot on an ephemeral port, hit `/api/setup`, shut down)

```go
func TestServeBootsAndServesSetup(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/console.yaml"
	ppath := dir + "/pipelock.yaml"
	_ = os.WriteFile(ppath, []byte("mode: block\n"), 0o600)
	_ = os.WriteFile(cfgPath, []byte("listen: \"127.0.0.1:0\"\nconfig_path: \""+ppath+"\"\n"), 0o600)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv, err := buildServer(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	resp, err := http.Get("http://" + ln.Addr().String() + "/api/setup")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("setup status = %d", resp.StatusCode)
	}
}
```

Add imports: `net`, `net/http`, `os`, `testing`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/pipelock-console/ -run TestServeBoots -v`
Expected: FAIL — `buildServer` undefined.

- [ ] **Step 3: Write the implementation** — replace the stub `runServe` and add `buildServer`:

```go
import (
	"net/http"

	consolecfg "github.com/luckyPipewrench/pipelock/internal/console/config"
	"github.com/luckyPipewrench/pipelock/internal/console/auth"
	"github.com/luckyPipewrench/pipelock/internal/console/configsvc"
	"github.com/luckyPipewrench/pipelock/internal/console/events"
	"github.com/luckyPipewrench/pipelock/internal/console/pipelockclient"
	"github.com/luckyPipewrench/pipelock/internal/console/server"
	"github.com/luckyPipewrench/pipelock/internal/console/service"
)

func buildServer(configPath string) (*http.Server, error) {
	cfg, err := consolecfg.Load(configPath)
	if err != nil {
		return nil, err
	}
	mgr := auth.NewManager(auth.Options{PasswordHash: cfg.AdminPasswordHash, SecretHex: cfg.SessionSecret})
	handler := server.New(server.Deps{
		Auth:    mgr,
		Config:  configsvc.New(cfg.ConfigPath),
		Client:  pipelockclient.New(pipelockclient.Options{BaseURL: cfg.Pipelock.BaseURL, KillswitchURL: cfg.Pipelock.KillswitchURL, APIToken: cfg.Pipelock.APIToken}),
		Service: service.New(cfg.ServiceUnit),
		Buffer:  events.NewBuffer(1000),
		Hub:     events.NewHub(),
		OnPasswordSet: func(hash string) {
			cfg.AdminPasswordHash = hash
			_ = consolecfg.Save(configPath, cfg)
		},
	})
	return &http.Server{Addr: cfg.Listen, Handler: handler}, nil
}

func runServe(_ *cobra.Command, configPath string) error {
	srv, err := buildServer(configPath)
	if err != nil {
		return err
	}
	cfg, _ := consolecfg.Load(configPath)
	if cfg != nil && cfg.TLS.CertFile != "" && cfg.TLS.KeyFile != "" {
		return srv.ListenAndServeTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile)
	}
	return srv.ListenAndServe()
}
```

Delete the temporary stub `runServe` from Task 2.

- [ ] **Step 4: Run tests, lint, commit**

Run: `gofumpt -w cmd/pipelock-console/ && golangci-lint run ./cmd/pipelock-console/... && go test ./cmd/pipelock-console/ -race -count=1 -v`
Expected: PASS.

```bash
git add cmd/pipelock-console/
git commit -m "feat(console): boot real server from serve command"
```

---

## Phase 7 — Frontend (Vite + React + Tailwind + shadcn, cyber-terminal theme)

> **Use the `frontend-design` skill for these tasks.** The backend is fully testable without the UI; these tasks build the embedded SPA. Each task ends by rebuilding (`make console-web`) and confirming the Go embed test still passes. Keep the "cyber-terminal" direction: near-black background, monospace, neon-green/red accents, subtle scanline texture.

### Task 18: Scaffold the frontend + Makefile build target + .gitignore

**Files:**
- Create: `internal/console/web/app/` (Vite project)
- Modify: `Makefile` (add `console`, `console-web` targets)
- Modify: `.gitignore` (ignore `internal/console/web/dist/` except keep nothing — build artifact)

- [ ] **Step 1: Scaffold Vite app**

```bash
cd internal/console/web
npm create vite@latest app -- --template react-ts
cd app
npm install
npm install -D tailwindcss postcss autoprefixer
npx tailwindcss init -p
npm install @codemirror/lang-yaml @uiw/react-codemirror @uiw/codemirror-theme-vscode
```

Configure `vite.config.ts` to build into `../dist`:
```ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
export default defineConfig({
  plugins: [react()],
  build: { outDir: "../dist", emptyOutDir: true },
  server: { proxy: { "/api": "http://127.0.0.1:9443", "/ingest": "http://127.0.0.1:9443" } },
});
```

Set up Tailwind (`tailwind.config.js` content globs, `index.css` with `@tailwind` directives and the cyber-terminal CSS variables). Initialize shadcn/ui per its docs.

- [ ] **Step 2: Add Makefile targets**

```makefile
.PHONY: console-web console
console-web:
	cd internal/console/web/app && npm ci && npm run build

console: console-web
	go build -trimpath $(LDFLAGS) -o pipelock-console ./cmd/pipelock-console
```

- [ ] **Step 3: Update `.gitignore`**

Add:
```
internal/console/web/dist/
internal/console/web/app/node_modules/
```

> Because dist is gitignored but `go:embed all:dist` needs files present, the committed placeholder `index.html` from Task 15 stays tracked with `git add -f` OR keep a tracked `dist/.gitkeep` + the placeholder. Confirm `go build ./cmd/pipelock-console` works on a clean checkout before `make console-web`. If not, force-add the placeholder: `git add -f internal/console/web/dist/index.html`.

- [ ] **Step 4: Build and verify embed test passes**

Run: `make console-web && go test ./internal/console/web/ -race -count=1 -v`
Expected: PASS (real index.html now embedded).

- [ ] **Step 5: Commit**

```bash
git add Makefile .gitignore internal/console/web/app/
git commit -m "feat(console): scaffold Vite React frontend and build targets"
```

---

### Task 19: App shell — sidebar nav + top bar (status + kill switch) + API client

**Files:**
- Create: `internal/console/web/app/src/api.ts` (typed fetch wrappers + SSE hook)
- Create: `internal/console/web/app/src/App.tsx` (router + shell)
- Create: `internal/console/web/app/src/components/Sidebar.tsx`, `TopBar.tsx`

- [ ] **Step 1:** Implement `api.ts` with functions: `getSetup`, `postSetup`, `login`, `logout`, `getStats`, `getSessions`, `getKillswitch`, `setKillswitch`, `getConfig`, `validateConfig`, `writeConfig`, `getService`, `restartService`, and `useEventStream()` (an `EventSource('/api/events')` React hook returning the rolling event list). All calls `credentials: "include"`; a 401 redirects to the login route.

- [ ] **Step 2:** Build the shell: left `Sidebar` (Overview / Events / Sessions / Config / Service), persistent `TopBar` showing pipelock health + mode and a red **kill-switch** toggle (calls `getKillswitch` on mount, `setKillswitch` on click with a confirm dialog). Client-side routing (react-router) with a login gate that checks `getSetup`/auth.

- [ ] **Step 3:** Apply the cyber-terminal theme tokens (mono font, near-black bg, neon accents, scanline overlay).

- [ ] **Step 4:** Build + verify.

Run: `make console-web && go test ./internal/console/web/ -race -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/console/web/app/src
git commit -m "feat(console): app shell, sidebar, top bar with kill switch, API client"
```

---

### Task 20: Overview screen

**Files:** Create `internal/console/web/app/src/screens/Overview.tsx`

- [ ] **Step 1:** Render counter cards (requests/24h via stats totals, blocked, flagged ≈ warn-level, active sessions), a "blocks by layer" list from `top_scanners`/`top_blocked_domains`, and a compact "recent events (live)" list driven by `useEventStream()`. Poll `getStats` every 5s.
- [ ] **Step 2:** Handle pipelock-down: show a banner ("pipelock unreachable") instead of crashing when `getStats` errors.
- [ ] **Step 3:** Build + verify embed test passes.
- [ ] **Step 4: Commit** `feat(console): overview screen`

---

### Task 21: Events screen (live, filterable)

**Files:** Create `internal/console/web/app/src/screens/Events.tsx`

- [ ] **Step 1:** Full-height table fed by `useEventStream()`: columns severity, type, time, target (from `fields`), with a severity filter (block/warn/info) and a free-text filter. Row click opens a detail drawer rendering the full `fields` JSON.
- [ ] **Step 2:** Cap rendered rows (e.g. last 500) to keep the DOM light; note this cap visibly ("showing last 500").
- [ ] **Step 3:** Build + verify.
- [ ] **Step 4: Commit** `feat(console): live events screen`

---

### Task 22: Sessions screen

**Files:** Create `internal/console/web/app/src/screens/Sessions.tsx`

- [ ] **Step 1:** Table from `getSessions` (poll every 5s); since the backend forwards raw session JSON, render key fields (key, tier, escalation level) defensively and a detail drawer with the raw object.
- [ ] **Step 2:** Empty + error states.
- [ ] **Step 3:** Build + verify.
- [ ] **Step 4: Commit** `feat(console): sessions screen`

---

### Task 23: Config screen (CodeMirror + validate + diff + write)

**Files:** Create `internal/console/web/app/src/screens/Config.tsx`

- [ ] **Step 1:** Load current YAML via `getConfig` into a CodeMirror 6 editor (`@uiw/react-codemirror` + `@codemirror/lang-yaml`, dark theme). A "Validate" button calls `validateConfig` and shows OK / the exact error / warnings inline. "Apply" calls `writeConfig`; on success show a toast ("written — pipelock will hot-reload") and reload the buffer; on rejection show the error and change nothing.
- [ ] **Step 2:** Show a diff (current vs edited) before Apply (a simple line diff is fine).
- [ ] **Step 3:** Add the structured quick-toggles panel above the editor: `mode` (block/warn/off select) and `enforce` (toggle), implemented by parsing/patching just those top-level keys and writing through the same validate+write path. Keep it minimal — the editor is the source of truth for everything else.
- [ ] **Step 4:** Build + verify.
- [ ] **Step 5: Commit** `feat(console): config editor with validation, diff, and quick-toggles`

---

### Task 24: Service screen + first-run password wizard + login page

**Files:** Create `internal/console/web/app/src/screens/Service.tsx`, `src/screens/Setup.tsx`, `src/screens/Login.tsx`

- [ ] **Step 1:** `Setup.tsx`: shown when `getSetup` returns `needs_setup: true`; a single "set admin password" form → `postSetup` → enters the app. `Login.tsx`: password form → `login`. Route guard: unauthenticated → Login; needs-setup → Setup.
- [ ] **Step 2:** `Service.tsx`: shows `getService` status (active/inactive), a "Restart pipelock" button (confirm dialog) → `restartService` with result/output, and displays console + pipelock version strings.
- [ ] **Step 3:** Build + verify.
- [ ] **Step 4: Commit** `feat(console): service screen, first-run wizard, login`

---

### Task 25: End-to-end smoke + docs + CI build wiring

**Files:**
- Create: `internal/console/server/e2e_test.go`
- Modify: `.github/workflows/*` (ensure Node is available so `make console` builds in CI; or build the web bundle in the release job)
- Create: `docs/console.md` (operator guide: console config, pipelock `emit.webhook` pointing at `/ingest`, `kill_switch.api_token` for the toggle, systemd permission note)

- [ ] **Step 1: Write an e2e test** that boots the full server with a fake pipelock (`httptest`) wired as `BaseURL`, logs in, posts an event to `/ingest`, and reads it back from `/api/events` (SSE) — proving the ingest→buffer→SSE path end to end.
- [ ] **Step 2:** Add a CI step (Node setup + `make console-web`) so the embedded build is exercised; confirm `govulncheck`/lint still pass for the new Go packages.
- [ ] **Step 3:** Write `docs/console.md`: how to configure `pipelock-console.yaml`, point pipelock's `emit.webhook` URL at `http://127.0.0.1:9443/ingest`, set `kill_switch.api_token` so the toggle works, and grant the console permission to run `systemctl restart pipelock` (run it under a unit/user that can, or a narrow sudoers line — operator's choice).
- [ ] **Step 4:** Run full suite: `go test -race -count=1 ./internal/console/... ./cmd/pipelock-console/...` and `golangci-lint run ./...`.
- [ ] **Step 5: Commit** `test(console): end-to-end ingest→SSE smoke + operator docs`

---

## Self-Review Notes

- **Spec coverage:** Observe (Tasks 6–11, 20–22), edit config (3–5, 23), kill switch (8, 19), service control (14, 24) — all covered. Single-password auth + first-run wizard (12–13, 24). Validate-before-reload (5). go:embed single binary (15, 18). Cyber-terminal UI (18–19). pipelock-down graceful degradation (7, 20). Event ingest via webhook (11) + SSE (10, 16).
- **Known follow-ups to confirm during implementation (not blockers):**
  - Resolved: `config.Warning` is `{Field, Message string}` (no `String()`) — Task 3 formats accordingly.
  - Resolved: `/api/v1/sessions` requires auth (`h.authenticate`), so `getJSON` (Task 6) sends the Bearer token unconditionally; `/stats` and `/health` are open and ignore it. Verify at integration that the console's `Pipelock.APIToken` matches pipelock's configured session/admin token.
  - Confirm pipelock `emit.webhook` POST shape matches `events.Event` (it does per `internal/emit/webhook.go`; re-check field names at integration time).
