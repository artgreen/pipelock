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

// runServe is replaced in Task 17 with the real server boot. Temporary stub
// so the package compiles and the serve command wiring can be tested.
func runServe(_ *cobra.Command, _ string) error { return nil }
