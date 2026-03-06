// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys wraps platform-specific syscall and signal handling.
// Implements prd002-sys.
package sys

import (
	"os"
	"os/signal"
	"syscall"
)

// InstallSIGPIPEHandler sets up SIGPIPE handling so that the process exits 0
// when a downstream consumer closes its stdin, matching GNU coreutils behavior.
func InstallSIGPIPEHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}
