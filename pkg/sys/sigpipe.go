// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys wraps platform-specific syscalls and signal handling (prd002-sys).
package sys

import (
	"os"
	"os/signal"
	"syscall"
)

// InstallSIGPIPEHandler causes the process to exit 0 on SIGPIPE, matching
// GNU coreutils behavior when piped output is consumed by a downstream
// consumer that closes its stdin.
func InstallSIGPIPEHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}
