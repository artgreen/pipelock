// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

// Package main is the entry point for the pipelock-console web app.
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/luckyPipewrench/pipelock/internal/console/auth"
	consolecfg "github.com/luckyPipewrench/pipelock/internal/console/config"
	"github.com/luckyPipewrench/pipelock/internal/console/configsvc"
	"github.com/luckyPipewrench/pipelock/internal/console/events"
	"github.com/luckyPipewrench/pipelock/internal/console/pipelockclient"
	"github.com/luckyPipewrench/pipelock/internal/console/server"
	"github.com/luckyPipewrench/pipelock/internal/console/service"
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

func buildServer(configPath string) (*http.Server, *consolecfg.ConsoleConfig, error) {
	cfg, err := consolecfg.Load(configPath)
	if err != nil {
		return nil, nil, err
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
	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv, cfg, nil
}

func runServe(_ *cobra.Command, configPath string) error {
	srv, cfg, err := buildServer(configPath)
	if err != nil {
		return err
	}
	if cfg.TLS.CertFile != "" && cfg.TLS.KeyFile != "" {
		return srv.ListenAndServeTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile)
	}
	return srv.ListenAndServe()
}
