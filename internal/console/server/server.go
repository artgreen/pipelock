// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package server wires console components into one HTTP handler.
package server

import (
	"encoding/json"
	"errors"
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
	Auth          *auth.Manager
	Config        *configsvc.Service
	Client        *pipelockclient.Client
	Service       *service.Controller
	Buffer        *events.Buffer
	Hub           *events.Hub
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
		_ = d.Auth.Login(w, pw) // best-effort auto-login; if it fails the user logs in manually
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
		var body struct {
			Active bool `json:"active"`
		}
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
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		writeJSON(w, configsvc.Validate(raw))
	})))
	mux.Handle("POST /api/config", d.Auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, readErr := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if readErr != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := d.Config.Write(raw); err != nil {
			var invalid *configsvc.InvalidConfigError
			if errors.As(err, &invalid) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	mux.Handle("GET /api/service", d.Auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// systemctl is-active exits non-zero for inactive/failed units while
		// still printing the status word; that is normal status, not an error.
		// Only treat an empty result with an error as a real failure.
		st, err := d.Service.Status(r.Context())
		if st == "" && err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
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

	fileServer := http.FileServerFS(web.FS())
	mux.Handle("GET /", spaFallback(fileServer))

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func decodePassword(w http.ResponseWriter, r *http.Request) string {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil || body.Password == "" {
		http.Error(w, "password required", http.StatusBadRequest)
		return ""
	}
	return body.Password
}
