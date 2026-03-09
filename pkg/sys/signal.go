// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"os/signal"
	"syscall"
)

// InstallSIGPIPEHandler installs a SIGPIPE handler that causes the process
// to exit with code 0 when a downstream pipe consumer closes its end.
// Uses signal.Notify with a buffered channel; a dedicated goroutine calls
// os.Exit(0) on receipt. Does not use signal.Ignore so deferred cleanup
// functions do not run on SIGPIPE, matching GNU coreutils behavior.
// Safe to call multiple times; each invocation starts one goroutine.
// (prd002-sys R1.5, R1.6, R1.7)
func InstallSIGPIPEHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}
