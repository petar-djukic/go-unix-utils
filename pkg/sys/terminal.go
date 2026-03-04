// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin || linux

// Implements prd002-sys R1.1, R1.2.

package sys

import "golang.org/x/sys/unix"

// TerminalWidth returns the terminal column count for the given file
// descriptor by calling the TIOCGWINSZ ioctl. Returns a non-nil error when
// fd does not refer to a terminal (pipe, regular file, etc.).
// R1.1: terminal width query via TIOCGWINSZ ioctl.
// R1.2: uses golang.org/x/sys/unix, not tput/stty/x/term.
func TerminalWidth(fd int) (int, error) {
	ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil {
		return 0, err
	}
	return int(ws.Col), nil
}
