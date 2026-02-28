// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// signal.go provides signal handling setup for SIGPIPE, SIGHUP, and SIGWINCH
// using Go's os/signal package.
//
// Implements: prd002-sys R3.1, R3.2, R4.1, R4.2.
package sys

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var sigpipeOnce sync.Once

// InstallSIGPIPEHandler installs a SIGPIPE handler causing the process to
// exit 0 when stdout or stderr is closed by a downstream consumer. Safe to
// call multiple times; only one handler goroutine is started.
//
// Implements: prd002-sys R3.1, R3.2.
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

// InstallSIGHUPHandler installs a SIGHUP handler that calls the provided
// callback when the signal is received. Each call registers a separate
// handler goroutine.
func InstallSIGHUPHandler(callback func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)
	go func() {
		<-c
		callback()
	}()
}

var (
	resizeMu             sync.Mutex
	resizeCallbacks      []func(width int)
	resizeHandlerStarted bool
)

// OnTerminalResize registers a callback that is invoked with the new terminal
// width when the terminal is resized (SIGWINCH). Multiple calls register
// multiple callbacks; callbacks are called in registration order. If
// TerminalSize returns an error during resize, the callback is not invoked.
//
// Implements: prd002-sys R4.1, R4.2.
func OnTerminalResize(callback func(width int)) {
	resizeMu.Lock()
	defer resizeMu.Unlock()

	resizeCallbacks = append(resizeCallbacks, callback)
	if !resizeHandlerStarted {
		resizeHandlerStarted = true
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGWINCH)
		go func() {
			for range c {
				cols, _, err := TerminalSize(os.Stdout.Fd())
				if err != nil {
					continue
				}
				resizeMu.Lock()
				cbs := make([]func(width int), len(resizeCallbacks))
				copy(cbs, resizeCallbacks)
				resizeMu.Unlock()
				for _, cb := range cbs {
					cb(cols)
				}
			}
		}()
	}
}
