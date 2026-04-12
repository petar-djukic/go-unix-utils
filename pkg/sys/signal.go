// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Signal handling utilities for pkg/sys.
// Implements srd002-sys R1.5 (InstallSIGPIPEHandler), R1.6 (idempotency).
package sys

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// sigpipeOnce ensures InstallSIGPIPEHandler installs exactly one handler
// regardless of how many times it is called. R1.6: safe to call multiple times.
var sigpipeOnce sync.Once

// installSIGPIPEHandlerImpl performs the actual signal handler installation.
// R1.5: uses signal.Notify with a buffered channel of size 1; a dedicated
// goroutine calls os.Exit(0) on receive. Does not use signal.Ignore so
// deferred cleanup functions do not run on SIGPIPE (matching GNU behavior).
func installSIGPIPEHandlerImpl() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}
