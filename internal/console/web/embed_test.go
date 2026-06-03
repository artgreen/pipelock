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
