// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd002-sys R1.5–R1.6 (InstallSIGPIPEHandler),
// R3.1–R3.2 (OnTerminalResize), and SIGHUP callback registration.

package sys

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// InstallSIGPIPEHandler installs a SIGPIPE handler that causes the process
// to exit with status 0 when SIGPIPE is received, matching GNU coreutils
// behavior. Each call starts a new goroutine with its own signal channel;
// safe to call multiple times. R1.5, R1.6.
func InstallSIGPIPEHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}

// sighupMu protects the sighupCallbacks slice.
var sighupMu sync.Mutex

// sighupCallbacks stores registered SIGHUP callbacks in registration order.
var sighupCallbacks []func()

// sighupOnce ensures the SIGHUP listener goroutine is started exactly once.
var sighupOnce sync.Once

// OnSIGHUP registers a callback that runs when SIGHUP is received.
// Multiple callbacks may be registered; they are called in registration order.
// The listener goroutine is started on the first call.
func OnSIGHUP(callback func()) {
	sighupMu.Lock()
	sighupCallbacks = append(sighupCallbacks, callback)
	sighupMu.Unlock()

	sighupOnce.Do(func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGHUP)
		go func() {
			for range c {
				sighupMu.Lock()
				cbs := make([]func(), len(sighupCallbacks))
				copy(cbs, sighupCallbacks)
				sighupMu.Unlock()
				for _, cb := range cbs {
					cb()
				}
			}
		}()
	})
}

// sigwinchMu protects the sigwinchCallbacks slice.
var sigwinchMu sync.Mutex

// sigwinchCallbacks stores registered SIGWINCH callbacks in registration order.
var sigwinchCallbacks []func(width int)

// sigwinchOnce ensures the SIGWINCH listener goroutine is started exactly once.
var sigwinchOnce sync.Once

// OnTerminalResize registers a callback that fires when SIGWINCH is received.
// The callback receives the new terminal width obtained via TerminalWidth().
// If TerminalWidth() returns an error, the callback is not invoked. R3.1.
// Multiple callbacks may be registered; they are called in registration order. R3.2.
func OnTerminalResize(callback func(width int)) {
	sigwinchMu.Lock()
	sigwinchCallbacks = append(sigwinchCallbacks, callback)
	sigwinchMu.Unlock()

	sigwinchOnce.Do(func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGWINCH)
		go func() {
			for range c {
				width, err := TerminalWidth()
				if err != nil {
					continue
				}
				sigwinchMu.Lock()
				cbs := make([]func(width int), len(sigwinchCallbacks))
				copy(cbs, sigwinchCallbacks)
				sigwinchMu.Unlock()
				for _, cb := range cbs {
					cb(width)
				}
			}
		}()
	})
}
