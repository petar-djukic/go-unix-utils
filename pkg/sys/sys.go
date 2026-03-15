// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R3.1-R3.3: OnTerminalResize SIGWINCH handler.

package sys

import (
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
)

// resizeCallback holds the current OnTerminalResize callback as an atomic value.
// R3.3: subsequent calls replace the previous callback rather than stacking.
var resizeCallback atomic.Pointer[func(width int)]

// resizeOnce ensures only one SIGWINCH listener goroutine is started.
// D3: single background goroutine; replacing the callback updates the atomic
// reference without spawning additional goroutines.
var resizeOnce sync.Once

// OnTerminalResize registers a SIGWINCH handler that calls the provided callback
// with the new terminal width when the terminal is resized. If TerminalWidth()
// returns an error the callback is not invoked.
//
// R3.1: registers SIGWINCH handler via signal.Notify.
// R3.2: callback receives the updated width from TerminalWidth().
// R3.3: safe to call multiple times; subsequent calls replace the previous
// callback rather than stacking handlers.
func OnTerminalResize(callback func(width int)) {
	resizeCallback.Store(&callback)

	resizeOnce.Do(func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGWINCH)
		go func() {
			for range c {
				cb := resizeCallback.Load()
				if cb == nil {
					continue
				}
				width, err := TerminalWidth()
				if err != nil {
					// R3.1: if TerminalWidth() returns an error, do not invoke callback.
					continue
				}
				(*cb)(width)
			}
		}()
	})
}
