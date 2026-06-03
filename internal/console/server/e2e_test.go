// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/luckyPipewrench/pipelock/internal/console/auth"
	"github.com/luckyPipewrench/pipelock/internal/console/configsvc"
	"github.com/luckyPipewrench/pipelock/internal/console/events"
	"github.com/luckyPipewrench/pipelock/internal/console/pipelockclient"
	"github.com/luckyPipewrench/pipelock/internal/console/server"
	"github.com/luckyPipewrench/pipelock/internal/console/service"
	"github.com/luckyPipewrench/pipelock/internal/testwait"
)

func TestEndToEndIngestToSSE(t *testing.T) {
	hash, _ := auth.HashPassword("pw")
	buf := events.NewBuffer(100)
	hub := events.NewHub()
	h := server.New(server.Deps{
		Auth:    auth.NewManager(auth.Options{PasswordHash: hash, SecretHex: "00112233445566778899aabbccddeeff"}),
		Config:  configsvc.New(t.TempDir() + "/pipelock.yaml"),
		Client:  pipelockclient.New(pipelockclient.Options{BaseURL: "http://127.0.0.1:1"}),
		Service: service.New("pipelock"),
		Buffer:  buf,
		Hub:     hub,
	})
	ts := httptest.NewServer(h)
	defer ts.Close()

	client := ts.Client()

	// login
	loginReq, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/api/login", strings.NewReader(`{"password":"pw"}`))
	loginResp, err := client.Do(loginReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = loginResp.Body.Close()
	cookies := loginResp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie")
	}

	// open SSE stream
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	sseReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	sseReq.AddCookie(cookies[0])
	sseResp, err := client.Do(sseReq)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sseResp.Body.Close() }()

	// Read the SSE stream in the background, capturing the first dlp.secret event.
	got := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(sseResp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") && strings.Contains(line, "dlp.secret") {
				got <- line
				return
			}
		}
	}()

	// Wait for the SSE handler to register its subscription before ingesting,
	// so the broadcast is guaranteed to reach this client.
	testwait.For(t, 5*time.Second, func() bool { return hub.SubscriberCount() == 1 }, "SSE subscriber to register")
	ingestReq, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/ingest", strings.NewReader(`{"severity":"critical","type":"dlp.secret","fields":{"target":"api.openai.com"}}`))
	ingestResp, err := client.Do(ingestReq)
	if err != nil {
		t.Fatal(err)
	}
	_ = ingestResp.Body.Close()

	select {
	case line := <-got:
		if !strings.Contains(line, "dlp.secret") {
			t.Errorf("unexpected SSE line: %q", line)
		}
	case <-ctx.Done():
		t.Fatal("event did not arrive on SSE stream within timeout")
	}
}
