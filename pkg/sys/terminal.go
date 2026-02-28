// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// terminal.go provides terminal size queries via TIOCGWINSZ ioctl and TTY
// detection using golang.org/x/sys/unix.
//
// Implements: prd002-sys R1.1, R1.2, R1.3.
package sys

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// defaultCols is the fallback column count when stdout is not a terminal.
const defaultCols = 80

// defaultRows is the fallback row count when stdout is not a terminal.
const defaultRows = 24

// TerminalSize queries the terminal dimensions (columns, rows) for the given
// file descriptor via TIOCGWINSZ ioctl. Returns an error when fd does not
// refer to a terminal or the ioctl fails.
//
// Implements: prd002-sys R1.1, R1.2.
func TerminalSize(fd uintptr) (cols, rows int, err error) {
	ws, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0, fmt.Errorf("querying terminal size: %w", err)
	}
	return int(ws.Col), int(ws.Row), nil
}

// TerminalWidth returns the current terminal column count by calling
// TerminalSize on stdout. Returns an error when stdout is not a terminal.
//
// Implements: prd002-sys R1.1.
func TerminalWidth() (int, error) {
	cols, _, err := TerminalSize(os.Stdout.Fd())
	return cols, err
}

// DefaultTerminalSize returns the terminal dimensions for stdout, falling back
// to 80 columns and 24 rows when stdout is not a terminal.
//
// Implements: prd002-sys R1.1.
func DefaultTerminalSize() (cols, rows int) {
	c, r, err := TerminalSize(os.Stdout.Fd())
	if err != nil {
		return defaultCols, defaultRows
	}
	return c, r
}

// IsTerminal reports whether the given file descriptor refers to a terminal.
// Returns false for pipes, regular files, and non-terminal descriptors.
// Implementation calls IoctlGetWinsize and returns err == nil.
//
// Implements: prd002-sys R1.3.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	return err == nil
}
