// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// TerminalWidth returns the current terminal column count by querying stdout
// via the TIOCGWINSZ ioctl. Returns an error when stdout is not a terminal
// or when the ioctl fails.
// See srd002-sys R1.1, R1.2.
func TerminalWidth() (int, error) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, fmt.Errorf("querying terminal width: %w", err)
	}
	return int(ws.Col), nil
}

// IsTerminal reports whether the file descriptor fd refers to a terminal.
// Returns false for pipes, regular files, and non-terminal descriptors.
// R1.3: uses IoctlGetWinsize with TIOCGWINSZ; err == nil means terminal.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	return err == nil
}

