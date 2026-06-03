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
		panic(err) // dist is embedded at build time; absence is a build error, not runtime input
	}
	return sub
}
