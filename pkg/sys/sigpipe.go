// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys wraps Darwin and Linux syscalls and signal handling.
// Implements prd002-sys.
package sys

import (
	"os"
	"os/signal"
	"syscall"
)

// InstallSIGPIPEHandler sets up a signal handler for SIGPIPE that causes
// the process to exit 0, matching GNU coreutils behavior when piped to a
// consumer that closes its stdin early (e.g., head, grep -q).
func InstallSIGPIPEHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}
