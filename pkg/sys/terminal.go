// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

// TerminalWidth returns the current terminal column count by calling
// ioctl(TIOCGWINSZ) on stdout.
//
// R1.1: returns an error when stdout is not a terminal or ioctl fails.
func TerminalWidth() (int, error) {
	panic("not implemented")
}

// IsTerminal returns true when the file descriptor refers to a terminal.
//
// R1.3: calls IoctlGetWinsize and returns err == nil.
func IsTerminal(fd uintptr) bool {
	panic("not implemented")
}

// OnTerminalResize registers a callback invoked with the new terminal width
// when SIGWINCH is received.
//
// R3.1: calls TerminalWidth and invokes callback with the result.
// R3.2: supports multiple registrations; callbacks are called in order.
func OnTerminalResize(callback func(width int)) {
	panic("not implemented")
}
