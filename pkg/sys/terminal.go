// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys wraps Darwin and Linux syscalls, providing a stable interface
// for cmd/ packages. Only syscalls that have platform divergence between
// Darwin and Linux, or that Go's standard library does not expose cleanly,
// belong here. cmd/ packages use pkg/sys instead of importing
// golang.org/x/sys/unix directly.
//
// Implements prd002-sys (R1, R2).
package sys

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// IsTerminal reports whether fd refers to a terminal device. It calls the
// TIOCGWINSZ ioctl via golang.org/x/sys/unix and returns true when the call
// succeeds. Returns false for pipes, regular files, and other non-terminal
// descriptors.
//
// prd002-sys R1.3.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	return err == nil
}

// GetWinSize queries the terminal dimensions (rows and columns) for the given
// file descriptor using the TIOCGWINSZ ioctl via golang.org/x/sys/unix.
// Returns an error when fd is not a terminal or the ioctl fails.
//
// prd002-sys R1.1, R1.2.
func GetWinSize(fd uintptr) (rows int, cols int, err error) {
	ws, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0, fmt.Errorf("querying terminal size: %w", err)
	}
	return int(ws.Row), int(ws.Col), nil
}
