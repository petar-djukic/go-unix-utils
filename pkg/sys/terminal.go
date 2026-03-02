// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// TerminalSize returns the current terminal dimensions (columns, rows) by
// issuing a TIOCGWINSZ ioctl on stdout. Returns an error when stdout is
// not a terminal or when the ioctl fails.
//
// Implements: prd002-sys R1.1, R1.2
func TerminalSize() (columns int, rows int, err error) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0, fmt.Errorf("querying terminal size: %w", err)
	}
	return int(ws.Col), int(ws.Row), nil
}
