// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd002-sys R2.1-R2.4
package sys

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var sigpipeOnce sync.Once

// InstallSIGPIPEHandler installs a SIGPIPE signal handler so the process exits
// with code 0 when writing to a closed pipe, matching GNU coreutils behavior.
// Safe to call multiple times from main(); subsequent calls are no-ops.
//
// R2.1: Install SIGPIPE handler that exits 0 on signal.
// R2.4: sync.Once guards installation so duplicate calls do not panic or spawn duplicate goroutines.
func InstallSIGPIPEHandler() {
	sigpipeOnce.Do(func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGPIPE)
		go func() {
			<-ch
			os.Exit(0)
		}()
	})
}

// OnTerminalResize registers a callback that is invoked with the new terminal
// width each time SIGWINCH is received. The callback receives the result of
// TerminalWidth(); if TerminalWidth returns an error the callback is not invoked
// for that signal. Each call registers an additional callback; all registered
// callbacks are invoked on each SIGWINCH.
//
// R2.2: Register callback invoked with new terminal width on SIGWINCH.
// R2.3: Query TerminalWidth(); skip callback on error.
func OnTerminalResize(callback func(width int)) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			w, err := TerminalWidth()
			if err != nil {
				continue
			}
			callback(w)
		}
	}()
}
