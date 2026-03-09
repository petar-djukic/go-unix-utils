// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

// TerminalWidth returns the current terminal column count by calling
// ioctl(TIOCGWINSZ) on stdout. R1.1, R1.2.
func TerminalWidth() (int, error) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, fmt.Errorf("terminal width: %w", err)
	}
	return int(ws.Col), nil
}

// IsTerminal returns true when the file descriptor refers to a terminal.
// R1.3: uses TIOCGWINSZ ioctl to detect terminal capability.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	return err == nil
}

// resizeMu protects the resizeCallbacks slice.
var resizeMu sync.Mutex

// resizeCallbacks holds registered SIGWINCH callbacks. R3.2.
var resizeCallbacks []func(width int)

// resizeOnce ensures only one SIGWINCH listener goroutine is started.
var resizeOnce sync.Once

// OnTerminalResize registers a callback invoked with the new terminal width
// when SIGWINCH is received. R3.1, R3.2: multiple callbacks supported,
// called in registration order.
func OnTerminalResize(callback func(width int)) {
	resizeMu.Lock()
	resizeCallbacks = append(resizeCallbacks, callback)
	resizeMu.Unlock()

	resizeOnce.Do(func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGWINCH)
		go func() {
			for range c {
				width, err := TerminalWidth()
				if err != nil {
					// R3.1: if TerminalWidth() returns an error, skip callbacks.
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
