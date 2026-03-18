// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R1.1–R1.3 (TerminalWidth, IsTerminal) and
// R2.2–R2.5 (OnTerminalResize SIGWINCH handler).
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
	resizeMu       sync.Mutex
	resizeCallback func(width int)
	resizeOnce     sync.Once
)

// OnTerminalResize registers a SIGWINCH handler that invokes the callback
// with the new terminal width when the terminal is resized. Subsequent calls
// replace the previous callback; only the most recently registered callback
// is invoked. If TerminalWidth returns an error, the callback is not invoked.
// The background listener goroutine is started on the first call and reused
// by subsequent calls. Implements prd002-sys R2.2–R2.5.
func OnTerminalResize(callback func(width int)) {
	resizeMu.Lock()
	resizeCallback = callback
	resizeMu.Unlock()

	resizeOnce.Do(func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGWINCH)
		go listenSIGWINCH(ch)
	})
}

// listenSIGWINCH waits for SIGWINCH signals and invokes the registered
// callback with the new terminal width.
func listenSIGWINCH(ch <-chan os.Signal) {
	for range ch {
		width, err := TerminalWidth()
		if err != nil {
			continue
		}
		resizeMu.Lock()
		cb := resizeCallback
		resizeMu.Unlock()
		if cb != nil {
			cb(width)
		}
	}
}
