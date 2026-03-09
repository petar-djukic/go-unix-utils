// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"os/signal"
	"syscall"
)

// InstallSIGPIPEHandler installs a signal handler that causes the process
// to exit 0 when SIGPIPE is received, matching GNU coreutils behavior.
// Uses signal.Notify (not signal.Ignore) so that deferred cleanup functions
// do not run on SIGPIPE. Safe to call multiple times; each invocation starts
// one goroutine. Implements prd002-sys R1.5, R1.6.
func InstallSIGPIPEHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}
