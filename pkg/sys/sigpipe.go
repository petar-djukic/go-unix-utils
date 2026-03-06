// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys wraps Darwin and Linux syscalls and signal handling (prd002-sys).
package sys

import (
	"os"
	"os/signal"
	"syscall"
)

// InstallSIGPIPEHandler sets up a SIGPIPE handler that exits 0,
// matching GNU coreutils behavior when piped output is closed.
func InstallSIGPIPEHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}
