// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R1.1-R1.3: TerminalWidth and IsTerminal functions.
// R1.7: ls, du, find, and xargs require terminal-width queries for layout.

package sys

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// TerminalWidth returns the current terminal column count by calling
// ioctl(TIOCGWINSZ) on stdout. Returns an error when stdout is not a
// terminal or when the ioctl call fails.
//
// R1.1: TerminalWidth returns (int, error), not panic.
// R1.2: Uses golang.org/x/sys/unix for the TIOCGWINSZ ioctl.
func TerminalWidth() (int, error) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, fmt.Errorf("sys.TerminalWidth: ioctl TIOCGWINSZ on stdout: %w", err)
	}
	return int(ws.Col), nil
}

// IsTerminal returns true when the file descriptor refers to a terminal,
// and false otherwise. Returns false for pipes, regular files, and
// non-terminal descriptors.
//
// R1.3: Implementation calls unix.IoctlGetWinsize and returns err == nil.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	return err == nil
}
