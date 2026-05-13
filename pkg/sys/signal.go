// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	resizeMu        sync.Mutex
	resizeCallbacks  []func(width int)
	resizeSetupOnce  sync.Once
)

// OnTerminalResize registers a callback invoked with the new terminal width
// when a SIGWINCH signal is received.
// R3.1: calls TerminalWidth() on SIGWINCH and invokes callback with the new width.
// R3.2: supports multiple callbacks called in registration order.
func OnTerminalResize(callback func(width int)) {
	resizeMu.Lock()
	resizeCallbacks = append(resizeCallbacks, callback)
	resizeMu.Unlock()

	resizeSetupOnce.Do(startResizeListener)
}

func startResizeListener() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGWINCH)
	go handleResizeSignals(c)
}

func handleResizeSignals(c <-chan os.Signal) {
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
}
