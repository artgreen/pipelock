// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package configsvc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateAcceptsGoodConfig(t *testing.T) {
	good := "mode: balanced\n"
	res := Validate([]byte(good))
	if !res.OK {
		t.Fatalf("expected valid, got error: %s", res.Error)
	}
}

func TestValidateRejectsUnknownField(t *testing.T) {
	bad := "mode: balanced\nbogus_field: true\n"
	res := Validate([]byte(bad))
	if res.OK {
		t.Fatal("expected invalid config to be rejected")
	}
	if !strings.Contains(strings.ToLower(res.Error), "bogus_field") {
		t.Errorf("error should name the offending field, got: %s", res.Error)
	}
}

func TestServiceReadsCurrentConfig(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/pipelock.yaml"
	if err := os.WriteFile(path, []byte("mode: balanced\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := New(path)
	got, err := svc.Read()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "mode: balanced\n" {
		t.Errorf("Read() = %q", got)
	}
}

func TestWriteRejectsInvalidAndChangesNothing(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/pipelock.yaml"
	original := "mode: audit\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := New(path)
	err := svc.Write([]byte("mode: balanced\nbogus_field: 1\n"))
	if err == nil {
		t.Fatal("expected write of invalid config to be rejected")
	}
	got, _ := os.ReadFile(filepath.Clean(path))
	if string(got) != original {
		t.Errorf("file mutated on rejected write: %q", got)
	}
	// fail-closed: no backup should be left behind by a rejected write
	backups, _ := filepath.Glob(path + ".bak.*")
	if len(backups) != 0 {
		t.Errorf("rejected write should not create a backup, found %d", len(backups))
	}
}

func TestWriteAppliesValidConfigWithBackup(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/pipelock.yaml"
	if err := os.WriteFile(path, []byte("mode: audit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := New(path)
	if err := svc.Write([]byte("mode: balanced\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, _ := os.ReadFile(filepath.Clean(path))
	if string(got) != "mode: balanced\n" {
		t.Errorf("new config not written: %q", got)
	}
	backups, _ := filepath.Glob(path + ".bak.*")
	if len(backups) != 1 {
		t.Errorf("expected exactly one backup, got %d", len(backups))
	}
	b, _ := os.ReadFile(backups[0])
	if string(b) != "mode: audit\n" {
		t.Errorf("backup should hold prior contents, got %q", b)
	}
}

// TestValidateRejectsFailOpenSecurityDefault is the regression for the
// fail-open drift bug: response_scanning is force-enabled (fail-closed) when
// `enabled` is omitted, so an invalid action must be rejected. A path that
// skipped applySecurityDefaults would wrongly report this config OK.
func TestValidateRejectsFailOpenSecurityDefault(t *testing.T) {
	bad := "mode: balanced\nresponse_scanning:\n  action: invalid_action\n"
	res := Validate([]byte(bad))
	if res.OK {
		t.Fatal("expected omitted-enabled response_scanning with bad action to be rejected (fail-closed)")
	}
	if !strings.Contains(strings.ToLower(res.Error), "response_scanning") {
		t.Errorf("error should name response_scanning, got: %s", res.Error)
	}
}
