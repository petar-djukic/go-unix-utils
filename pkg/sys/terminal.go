// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"fmt"
	"os"
	"os/signal"
	"sync"

	"golang.org/x/sys/unix"
)

// TerminalWidth returns the current terminal column count by calling
// ioctl(TIOCGWINSZ) on stdout. Returns an error when stdout is not a
// terminal or when ioctl fails. Implements prd002-sys R1.1, R1.2.
func TerminalWidth() (int, error) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, fmt.Errorf("ioctl TIOCGWINSZ: %w", err)
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

// resizeMu protects the resizeCallbacks slice.
var (
	resizeMu        sync.Mutex
	resizeCallbacks []func(width int)
	resizeOnce      sync.Once
)

// OnTerminalResize registers a callback that is invoked with the new
// terminal width when SIGWINCH is received. Multiple calls register
// multiple callbacks; they are invoked in registration order. If
// TerminalWidth returns an error on resize, the callback is not invoked.
// Implements prd002-sys R3.1, R3.2.
func OnTerminalResize(callback func(width int)) {
	resizeMu.Lock()
	resizeCallbacks = append(resizeCallbacks, callback)
	resizeMu.Unlock()

	resizeOnce.Do(func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, unix.SIGWINCH)
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
	})
}
