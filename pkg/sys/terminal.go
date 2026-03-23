// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R1.1–R1.3, R3.1–R3.2: TerminalWidth, IsTerminal,
// and OnTerminalResize.
package sys

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

// stdoutFD is the file descriptor for stdout, used by TerminalWidth.
const stdoutFD = 1

// resizeMu protects resizeCallbacks and resizeRegistered.
var resizeMu sync.Mutex

// resizeCallbacks holds callbacks registered via OnTerminalResize.
// R3.2: callbacks are called in registration order.
var resizeCallbacks []func(width int)

// resizeRegistered tracks whether the SIGWINCH goroutine has been started.
var resizeRegistered bool

// TerminalWidth returns the width of the controlling terminal in columns.
//
// R1.1: queries TIOCGWINSZ ioctl on stdout (fd 1).
// R1.2: uses golang.org/x/sys/unix, not tput/stty/x/term.
// R2.5: returns a sensible error when stdout is redirected to a pipe or file.
func TerminalWidth() (int, error) {
	ws, err := unix.IoctlGetWinsize(stdoutFD, unix.TIOCGWINSZ)
	if err != nil {
		return 0, fmt.Errorf("querying terminal width: %w", err)
	}
	return int(ws.Col), nil
}

// IsTerminal reports whether the given file descriptor refers to a terminal.
//
// R1.3: attempts TIOCGWINSZ ioctl; a nil error means the fd is a terminal.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	return err == nil
}

// OnTerminalResize registers a callback that is invoked whenever the terminal
// is resized (SIGWINCH). The callback receives the new terminal width.
//
// R3.1: on SIGWINCH, calls TerminalWidth and invokes the callback with the
// new width. If TerminalWidth returns an error, the callback is not invoked.
// R3.2: supports multiple calls; each registered callback is called in
// registration order.
func OnTerminalResize(callback func(width int)) {
	resizeMu.Lock()
	defer resizeMu.Unlock()

	resizeCallbacks = append(resizeCallbacks, callback)

	if !resizeRegistered {
		resizeRegistered = true
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGWINCH)
		go handleResize(c)
	}
}

// handleResize listens for SIGWINCH signals and invokes all registered
// callbacks with the new terminal width.
func handleResize(c <-chan os.Signal) {
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
}
