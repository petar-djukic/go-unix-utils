// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R1.1, R1.2, R1.3: terminal width queries via
// TIOCGWINSZ ioctl using golang.org/x/sys/unix.

package sys

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// TerminalWidth returns the current terminal column count by calling
// ioctl(TIOCGWINSZ) on stdout. Returns an error when stdout is not a
// terminal or when the ioctl fails. (prd002-sys R1.1, R1.2)
func TerminalWidth() (int, error) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, fmt.Errorf("ioctl TIOCGWINSZ on stdout: %w", err)
	}
	return int(ws.Col), nil
}

// IsTerminal reports whether fd refers to a terminal. Returns false for
// pipes, regular files, and non-terminal descriptors. Implementation uses
// golang.org/x/sys/unix.IoctlGetWinsize as specified; a successful ioctl
// indicates a terminal. (prd002-sys R1.3)
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	return err == nil
}
