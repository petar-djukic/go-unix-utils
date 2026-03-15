// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R1.5-R1.6: InstallSIGPIPEHandler.
// Implements prd002-sys R3.1-R3.2: OnTerminalResize with SIGWINCH handling.

package sys

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// sigpipeOnce ensures InstallSIGPIPEHandler starts only one goroutine
// regardless of how many times it is called.
// R1.6: safe to call multiple times.
var sigpipeOnce sync.Once

// InstallSIGPIPEHandler installs a SIGPIPE signal handler that causes the
// process to exit with status 0 when SIGPIPE is received, matching GNU
// coreutils behavior when piped to consumers that close stdin early.
//
// R1.5: Uses signal.Notify with a buffered channel of size 1. A dedicated
// goroutine calls os.Exit(0) when the signal is received. Does not use
// signal.Ignore so that deferred cleanup functions do not run on SIGPIPE.
// R1.6: Safe to call multiple times; only one goroutine is started.
func InstallSIGPIPEHandler() {
	sigpipeOnce.Do(func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGPIPE)
		go func() {
			<-c
			os.Exit(0)
		}()
	})
}

// winchMu protects winchCallbacks.
var winchMu sync.Mutex

// winchCallbacks holds registered SIGWINCH callbacks in registration order.
var winchCallbacks []func(int)

// winchOnce ensures the SIGWINCH listener goroutine is started at most once.
var winchOnce sync.Once

// OnTerminalResize registers a callback that is invoked when the terminal is
// resized (SIGWINCH). The callback receives the new terminal width obtained by
// calling TerminalWidth(). If TerminalWidth() returns an error, the callback
// is not invoked.
//
// R3.1: Registers a SIGWINCH handler via signal.Notify. The goroutine runs
// for the lifetime of the process.
// R3.2: Multiple callbacks are supported; each is called in registration order.
func OnTerminalResize(callback func(width int)) {
	winchMu.Lock()
	winchCallbacks = append(winchCallbacks, callback)
	winchMu.Unlock()

	// Start the listener goroutine on the first call.
	winchOnce.Do(startWinchListener)
}

// startWinchListener starts a goroutine that listens for SIGWINCH signals and
// invokes all registered callbacks. Called once via winchOnce.
func startWinchListener() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGWINCH)

	go func() {
		for range c {
			width, err := TerminalWidth()
			if err != nil {
				continue
			}
			winchMu.Lock()
			// Copy the slice to release the lock before calling callbacks.
			cbs := make([]func(int), len(winchCallbacks))
			copy(cbs, winchCallbacks)
			winchMu.Unlock()

			for _, cb := range cbs {
				cb(width)
			}
		}
	}()
}
