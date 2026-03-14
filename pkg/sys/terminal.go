// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd002-sys R1.5-R1.7
package sys

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// TerminalWidth returns the number of columns in the terminal attached to stdout.
// It queries the terminal size via the TIOCGWINSZ ioctl. If stdout is not a
// terminal, an error wrapping the underlying syscall error is returned.
//
// R1.5: TerminalWidth queries terminal column width via TIOCGWINSZ ioctl.
// R1.7: TerminalWidth returns an error when stdout is not a terminal.
func TerminalWidth() (int, error) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, fmt.Errorf("TerminalWidth: %w", err)
	}
	return int(ws.Col), nil
}

// IsTerminal reports whether the given file descriptor refers to a terminal.
// It uses the TIOCGWINSZ ioctl; the ioctl returns ENOTTY when fd is not a terminal.
//
// R1.6: IsTerminal returns true when fd refers to a terminal.
// R1.7: IsTerminal returns false for non-terminal file descriptors.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	return err == nil
}
