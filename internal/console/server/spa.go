// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"
	"strings"

	"github.com/luckyPipewrench/pipelock/internal/console/web"
)

// spaFallback serves static files, falling back to index.html for non-asset
// paths so client-side routing works on deep links.
func spaFallback(fileServer http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if f, err := web.FS().Open(strings.TrimPrefix(r.URL.Path, "/")); err != nil && r.URL.Path != "/" {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		} else if err == nil {
			_ = f.Close()
		}
		fileServer.ServeHTTP(w, r)
	})
}
