// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"os/signal"
	"syscall"
)

// InstallSIGPIPEHandler sets up a SIGPIPE signal handler so that utilities
// writing to stdout exit cleanly when piped to a consumer that closes stdin
// early, matching GNU coreutils behavior.
//
// R1.5: uses signal.Notify with a buffered channel of size 1; a dedicated
// goroutine calls os.Exit(0) when the signal is received. Does not use
// signal.Ignore so deferred cleanup functions do not run on SIGPIPE.
// R1.6: safe to call multiple times; each invocation starts one goroutine.
func InstallSIGPIPEHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}
