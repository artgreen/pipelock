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
	return exec.CommandContext(ctx, name, args...).CombinedOutput() //nolint:gosec // G204: fixed argv (systemctl <verb> <unit>); unit is operator config, no shell
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
