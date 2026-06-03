// Copyright 2026 Josh Waldrep
// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/luckyPipewrench/pipelock/internal/audit"
)

// safeClose calls Close on c and logs any unexpected error via the audit logger.
// The label identifies the resource in log messages (e.g. "targetConn",
// "resp.Body"). If logger is nil, errors are silently discarded.
//
// Benign "already closed" results (net.ErrClosed, io.ErrClosedPipe) are not
// logged: during proxy teardown a connection is routinely closed from more than
// one path (a deferred close plus an explicit error-path close), so the later
// Close returns "use of closed network connection". That is expected, not a
// fault — logging it at error level is noise. Genuine close failures still log.
//
// Use this instead of bare close-and-ignore patterns in proxy code so close
// failures are observable in audit logs.
func safeClose(c io.Closer, label string, logger *audit.Logger) {
	if c == nil {
		return
	}
	err := c.Close()
	if err == nil || logger == nil {
		return
	}
	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) {
		return
	}
	logger.LogError(audit.NewMethodLogContext("close"), fmt.Errorf("%s: %w", label, err))
}
