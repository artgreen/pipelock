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

func TestNewUsesRealRunner(t *testing.T) {
	c := New("pipelock")
	if c.Unit != "pipelock" || c.run == nil {
		t.Error("New should set Unit and a non-nil runner")
	}
}
