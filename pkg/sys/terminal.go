// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R1.1–R1.3 (TerminalWidth, IsTerminal) and
// R3.1–R3.3 (OnTerminalResize SIGWINCH handler with multiple callbacks).
package sys

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

// TerminalWidth returns the current terminal column count by querying
// TIOCGWINSZ on stdout. Returns an error when stdout is not a terminal
// or when the ioctl fails. Implements prd002-sys R1.1.
func TerminalWidth() (int, error) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, err
	}
	return int(ws.Col), nil
}

// IsTerminal returns true when the file descriptor refers to a terminal,
// and false otherwise. Returns false for pipes, regular files, and
// non-terminal descriptors. Implements prd002-sys R1.3.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	return err == nil
}

var (
	resizeMu        sync.Mutex
	resizeCallbacks []func(width int)
	resizeOnce      sync.Once
)

// OnTerminalResize registers a SIGWINCH handler that invokes the callback
// with the new terminal width when the terminal is resized. Multiple calls
// accumulate callbacks; all registered callbacks are invoked in registration
// order on each resize. If TerminalWidth returns an error, no callback is
// invoked. The background listener goroutine is started on the first call
// and reused by subsequent calls.
// Implements prd002-sys R3.1–R3.3.
func OnTerminalResize(callback func(width int)) {
	resizeMu.Lock()
	resizeCallbacks = append(resizeCallbacks, callback)
	resizeMu.Unlock()

	resizeOnce.Do(func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGWINCH)
		go listenSIGWINCH(ch)
	})
}

// listenSIGWINCH waits for SIGWINCH signals and invokes all registered
// callbacks with the new terminal width in registration order.
func listenSIGWINCH(ch <-chan os.Signal) {
	for range ch {
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
}
