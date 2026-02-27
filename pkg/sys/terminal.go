// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys wraps Darwin and Linux syscalls and signal handling, providing a
// stable interface that cmd/ packages use to avoid platform-specific code in
// utility implementations.
//
// Implements: prd002-sys (R1, R2, R3, R4)
package sys

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// TerminalWidth returns the current terminal column count by calling
// ioctl(TIOCGWINSZ) on stdout. Returns a non-nil error when stdout is not a
// terminal or when the ioctl fails.
//
// Per prd002-sys R1.1-R1.2.
// Utility context: ls uses terminal width for multi-column layout (ls.c:2281).
func TerminalWidth() (int, error) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, fmt.Errorf("querying terminal width: %w", err)
	}
	return int(ws.Col), nil
}

// IsTerminal returns true when the file descriptor refers to a terminal. Uses
// golang.org/x/sys/unix for the TTY check via TIOCGWINSZ ioctl.
//
// Required by pkg/format (prd003-format R2.3) for ColorEnabled.
// Per prd002-sys R1.3: must not import golang.org/x/term.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	return err == nil
}
