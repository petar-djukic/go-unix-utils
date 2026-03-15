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

// winchMu protects winchCallbacks and winchCancel.
var winchMu sync.Mutex

// winchCallbacks holds registered SIGWINCH callbacks in registration order.
// Each entry has a unique ID for cancel/deregistration.
var winchCallbacks []winchEntry

// winchCancel holds the signal.Stop function for the current SIGWINCH channel.
// nil when no listener goroutine is running.
var winchCancel func()

// winchEntry pairs a callback with a unique ID for deregistration.
type winchEntry struct {
	id       uint64
	callback func(int)
}

// winchNextID is the next callback ID to assign.
var winchNextID uint64

// OnTerminalResize registers a callback that is invoked when the terminal is
// resized (SIGWINCH). The callback receives the new terminal width obtained by
// calling TerminalWidth(). If TerminalWidth() returns an error, the callback
// is not invoked.
//
// R3.1: Registers a SIGWINCH handler via signal.Notify.
// R3.2: Multiple callbacks are supported; each is called in registration order.
//
// Returns a cancel function that deregisters the callback. When all callbacks
// are deregistered, the SIGWINCH listener goroutine is stopped.
func OnTerminalResize(callback func(width int)) func() {
	winchMu.Lock()
	defer winchMu.Unlock()

	id := winchNextID
	winchNextID++

	winchCallbacks = append(winchCallbacks, winchEntry{id: id, callback: callback})

	// Start the listener goroutine if this is the first callback.
	if winchCancel == nil {
		startWinchListener()
	}

	return func() {
		removeWinchCallback(id)
	}
}

// startWinchListener starts a goroutine that listens for SIGWINCH signals and
// invokes all registered callbacks. Must be called with winchMu held.
func startWinchListener() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGWINCH)

	done := make(chan struct{})
	winchCancel = func() {
		signal.Stop(c)
		close(done)
	}

	go func() {
		for {
			select {
			case <-done:
				return
			case <-c:
				width, err := TerminalWidth()
				if err != nil {
					continue
				}
				winchMu.Lock()
				// Copy the slice to release the lock before calling callbacks.
				cbs := make([]winchEntry, len(winchCallbacks))
				copy(cbs, winchCallbacks)
				winchMu.Unlock()

				for _, entry := range cbs {
					entry.callback(width)
				}
			}
		}
	}()
}

// removeWinchCallback removes a callback by ID. If no callbacks remain, stops
// the SIGWINCH listener goroutine.
func removeWinchCallback(id uint64) {
	winchMu.Lock()
	defer winchMu.Unlock()

	for i, entry := range winchCallbacks {
		if entry.id == id {
			winchCallbacks = append(winchCallbacks[:i], winchCallbacks[i+1:]...)
			break
		}
	}

	// Stop the listener if no callbacks remain.
	if len(winchCallbacks) == 0 && winchCancel != nil {
		winchCancel()
		winchCancel = nil
	}
}
