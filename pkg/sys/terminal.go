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
// ioctl(TIOCGWINSZ) on stdout.
// R1.1: returns an error when stdout is not a terminal or ioctl fails.
// R1.2: uses golang.org/x/sys/unix for the ioctl call.
func TerminalWidth() (int, error) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, fmt.Errorf("terminal width: %w", err)
	}
	return int(ws.Col), nil
}

// IsTerminal returns true when the file descriptor refers to a terminal device.
// R1.3: calls IoctlGetWinsize and returns err == nil.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	return err == nil
}

// resizeCallbacks holds registered SIGWINCH callbacks.
// R3.2: supports multiple callbacks called in registration order.
var (
	resizeMu        sync.Mutex
	resizeCallbacks []func(width int)
	resizeOnce      sync.Once
)

// OnTerminalResize registers a callback invoked with the new terminal width
// whenever SIGWINCH is received.
// R3.1: queries TerminalWidth on each SIGWINCH; skips callback if it errors.
// R3.2: multiple callbacks are supported and called in registration order.
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
