// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

// TerminalWidth returns the current terminal column count by calling
// ioctl(TIOCGWINSZ) on stdout. Returns an error when stdout is not a terminal
// or when ioctl fails. (prd002-sys R1.1, R1.2)
func TerminalWidth() (int, error) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, err
	}
	return int(ws.Col), nil
}

// IsTerminal returns true when the file descriptor refers to a terminal, and
// false otherwise. Returns false for pipes, regular files, and non-terminal
// descriptors. (prd002-sys R1.3)
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	return err == nil
}

var (
	resizeMu        sync.Mutex
	resizeCallbacks []func(width int)
	resizeOnce      sync.Once
)

// OnTerminalResize registers a callback that is invoked with the new terminal
// width when the terminal is resized (SIGWINCH). Multiple calls register
// multiple callbacks; they are called in registration order. If TerminalWidth
// returns an error, the callback is not invoked. (prd002-sys R3.1, R3.2)
func OnTerminalResize(callback func(width int)) {
	resizeMu.Lock()
	resizeCallbacks = append(resizeCallbacks, callback)
	resizeMu.Unlock()

	resizeOnce.Do(func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGWINCH)
		go func() {
			for range c {
				w, err := TerminalWidth()
				if err != nil {
					continue
				}
				resizeMu.Lock()
				cbs := make([]func(width int), len(resizeCallbacks))
				copy(cbs, resizeCallbacks)
				resizeMu.Unlock()
				for _, cb := range cbs {
					cb(w)
				}
			}
		}()
	})
}
