// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R1.5, R1.6, R3.1, R3.2: signal handler registration
// for SIGPIPE, SIGHUP, and SIGWINCH.

package sys

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	sigpipeOnce sync.Once

	resizeMu        sync.Mutex
	resizeCallbacks []func(int)
	resizeOnce      sync.Once
)

// InstallSIGPIPEHandler installs a SIGPIPE handler that causes the process to
// exit 0 when stdout or stderr is closed by a downstream consumer. Uses
// signal.Notify with a buffered channel of size 1 so the goroutine does not
// miss rapid signals. Safe to call multiple times; only one goroutine is
// started. (prd002-sys R1.5, R1.6)
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

// InstallSIGHUPHandler registers callback to be called each time the process
// receives SIGHUP. Multiple registrations via separate calls each receive
// their own notification channel and goroutine.
func InstallSIGHUPHandler(callback func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)
	go func() {
		for range c {
			callback()
		}
	}()
}

// OnTerminalResize registers callback to be invoked with the new terminal
// column count when SIGWINCH is received. Multiple callbacks registered via
// successive calls are all invoked in registration order. The SIGWINCH
// goroutine is started exactly once regardless of how many callbacks are
// registered. If TerminalWidth() returns an error on resize, the callbacks
// are not called. (prd002-sys R3.1, R3.2)
func OnTerminalResize(callback func(width int)) {
	resizeMu.Lock()
	resizeCallbacks = append(resizeCallbacks, callback)
	resizeMu.Unlock()

	resizeOnce.Do(func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGWINCH)
		go func() {
			for range c {
				width, err := TerminalWidth()
				if err != nil {
					continue
				}
				resizeMu.Lock()
				cbs := make([]func(int), len(resizeCallbacks))
				copy(cbs, resizeCallbacks)
				resizeMu.Unlock()
				for _, cb := range cbs {
					cb(width)
				}
			}
		}()
	})
}
