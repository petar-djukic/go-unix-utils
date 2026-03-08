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

// TerminalWidth returns the current terminal column count by querying
// TIOCGWINSZ on stdout. R1.1, R1.2.
func TerminalWidth() (int, error) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, fmt.Errorf("terminal width: %w", err)
	}
	return int(ws.Col), nil
}

// IsTerminal returns true when the file descriptor refers to a terminal. R1.3.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	return err == nil
}

// resizeMu protects the resizeCallbacks slice.
var resizeMu sync.Mutex

// resizeCallbacks holds registered SIGWINCH callbacks.
var resizeCallbacks []func(width int)

// resizeOnce ensures the SIGWINCH listener goroutine starts only once.
var resizeOnce sync.Once

// OnTerminalResize registers a callback invoked with the new terminal width
// when a SIGWINCH signal is received. R3.1, R3.2.
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
