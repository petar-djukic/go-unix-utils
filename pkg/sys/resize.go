// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Terminal resize handling for pkg/sys.
// Implements srd002-sys R3.1 (OnTerminalResize), R3.2 (multiple callbacks).
package sys

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// resizeMu protects resizeCallbacks and resizeStarted.
var resizeMu sync.Mutex

// resizeCallbacks holds registered resize callbacks in registration order.
// R3.2: each registered callback is called on resize.
var resizeCallbacks []func(width int)

// resizeStarted tracks whether the SIGWINCH listener goroutine has been launched.
var resizeStarted bool

// startResizeListener launches the background SIGWINCH goroutine.
// R2.6: runs in a background goroutine and does not block the caller.
func startResizeListener() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGWINCH)
	go func() {
		for range c {
			dispatchResize()
		}
	}()
}

// dispatchResize reads the current terminal width and invokes all registered
// callbacks. R3.1: if TerminalWidth() returns an error, callbacks are not invoked.
func dispatchResize() {
	width, err := TerminalWidth()
	if err != nil {
		return
	}
	resizeMu.Lock()
	cbs := make([]func(width int), len(resizeCallbacks))
	copy(cbs, resizeCallbacks)
	resizeMu.Unlock()
	for _, cb := range cbs {
		cb(width)
	}
}

// OnTerminalResize registers a callback that is invoked with the new terminal
// width whenever SIGWINCH is received. Multiple callbacks may be registered;
// they are called in registration order. See srd002-sys R3.1, R3.2.
func OnTerminalResize(callback func(width int)) {
	resizeMu.Lock()
	defer resizeMu.Unlock()
	resizeCallbacks = append(resizeCallbacks, callback)
	if !resizeStarted {
		resizeStarted = true
		startResizeListener()
	}
}

// TODO: Setpgid and Killpg process group wrappers requested by task R3 but
// listed in srd002-sys non_goals: "pkg/sys does not provide process-group or
// terminal control (TIOCGPGRP, tcsetpgrp); those are deferred to the xargs SRD."
// Per constitution E6, skipping implementation. See srd002-sys non_goals.
