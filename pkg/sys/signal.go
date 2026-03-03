// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// InstallSIGPIPEHandler installs a SIGPIPE handler that causes the process to
// exit 0 when stdout or stderr is closed by a downstream consumer. This matches
// GNU behavior where SIGPIPE terminates silently (prd002-sys R1.5, R1.6).
//
// Safe to call multiple times; each invocation starts exactly one goroutine.
func InstallSIGPIPEHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}

var (
	resizeMu        sync.Mutex
	resizeCallbacks []func(width int)
	resizeInstalled bool
)

// OnTerminalResize registers a callback invoked with the new terminal width
// when a SIGWINCH signal is received. Multiple callbacks may be registered;
// they are called in registration order. If TerminalWidth returns an error
// the callback is not invoked (prd002-sys R3.1, R3.2).
func OnTerminalResize(callback func(width int)) {
	resizeMu.Lock()
	defer resizeMu.Unlock()

	resizeCallbacks = append(resizeCallbacks, callback)

	if !resizeInstalled {
		resizeInstalled = true
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGWINCH)
		go func() {
			for range c {
				width, err := TerminalWidth()
				if err != nil {
					continue
				}
				resizeMu.Lock()
				cbs := make([]func(width int), len(resizeCallbacks))
				copy(cbs, resizeCallbacks)
				resizeMu.Unlock()
				for _, cb := range cbs {
					cb(width)
				}
			}
		}()
	}
}
