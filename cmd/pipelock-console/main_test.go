// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"net"
	"net/http"
	"os"
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

func TestServeBootsAndServesSetup(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/console.yaml"
	ppath := dir + "/pipelock.yaml"
	_ = os.WriteFile(ppath, []byte("mode: audit\n"), 0o600)
	_ = os.WriteFile(cfgPath, []byte("listen: \"127.0.0.1:0\"\nconfig_path: \""+ppath+"\"\n"), 0o600)

	srv, _, err := buildServer(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	lc := net.ListenConfig{}
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+ln.Addr().String()+"/api/setup", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("setup status = %d", resp.StatusCode)
	}
}
