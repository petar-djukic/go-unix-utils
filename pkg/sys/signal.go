// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// InstallSIGPIPEHandler installs a signal handler that calls os.Exit(0) when
// SIGPIPE is received. This suppresses the broken-pipe error when writing to a
// closed stdout or stderr, matching GNU behavior where SIGPIPE terminates
// silently. Uses a real signal handler (not signal.Ignore) so that deferred
// cleanup functions do not run.
//
// Per prd002-sys R3.1-R3.2.
// Utility context: ls, du, find, and xargs install SIGPIPE handlers to
// suppress broken-pipe errors when piped to head or other truncating consumers.
func InstallSIGPIPEHandler() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGPIPE)
	go func() {
		<-ch
		os.Exit(0)
	}()
}

// resizeState holds the registered SIGWINCH callbacks and installation state.
var resizeState struct {
	mu        sync.Mutex
	callbacks []func(width int)
	installed bool
}

// OnTerminalResize registers a callback that is invoked with the new terminal
// width when a SIGWINCH signal is received (terminal resize). Multiple calls to
// OnTerminalResize are supported; each registered callback is called on resize.
//
// Per prd002-sys R4.1-R4.2.
// Utility context: ls re-queries terminal width on SIGWINCH to re-format output
// when the terminal window is resized during a long listing (ls.c:2281).
func OnTerminalResize(callback func(width int)) {
	resizeState.mu.Lock()
	defer resizeState.mu.Unlock()

	resizeState.callbacks = append(resizeState.callbacks, callback)

	if !resizeState.installed {
		resizeState.installed = true
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGWINCH)
		go func() {
			for range ch {
				width, err := TerminalWidth()
				if err != nil {
					// Terminal width unavailable; skip callback invocation.
					continue
				}
				resizeState.mu.Lock()
				cbs := make([]func(int), len(resizeState.callbacks))
				copy(cbs, resizeState.callbacks)
				resizeState.mu.Unlock()
				for _, cb := range cbs {
					cb(width)
				}
			}
		}()
	}
}
