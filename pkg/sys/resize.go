// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// SIGWINCH handling for terminal resize events.
// Implements prd002-sys R3.1, R3.2, R3.3.
package sys

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// resizeState holds the process-global SIGWINCH callback registry.
// Protected by mu; the signal listener goroutine is started once.
var resizeState struct {
	mu        sync.Mutex
	callbacks []func(width int)
	started   bool
}

// OnTerminalResize registers a callback that is invoked with the new terminal
// width when a SIGWINCH signal is received. If TerminalWidth() returns an error
// at the time of the signal, the callback is not invoked. Multiple calls to
// OnTerminalResize register additional callbacks; all are called in registration
// order on each resize event. (prd002-sys R3.1, R3.2)
func OnTerminalResize(callback func(width int)) {
	resizeState.mu.Lock()
	defer resizeState.mu.Unlock()

	resizeState.callbacks = append(resizeState.callbacks, callback)

	if !resizeState.started {
		resizeState.started = true
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGWINCH)
		go func() {
			for range c {
				width, err := TerminalWidth()
				if err != nil {
					continue
				}
				resizeState.mu.Lock()
				cbs := make([]func(width int), len(resizeState.callbacks))
				copy(cbs, resizeState.callbacks)
				resizeState.mu.Unlock()
				for _, cb := range cbs {
					cb(width)
				}
			}
		}()
	}
}
