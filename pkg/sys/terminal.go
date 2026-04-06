// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

// TerminalWidth returns the current terminal column count by querying stdout.
// Returns an error when stdout is not a terminal or when the ioctl fails.
// See srd002-sys R1.1.
func TerminalWidth() (int, error) {
	panic("not implemented")
}

// IsTerminal reports whether the file descriptor fd refers to a terminal.
// Returns false for pipes, regular files, and non-terminal descriptors.
// See srd002-sys R1.3.
func IsTerminal(fd uintptr) bool {
	panic("not implemented")
}

// OnTerminalResize registers a callback that is invoked with the new terminal
// width whenever SIGWINCH is received. Multiple callbacks may be registered;
// they are called in registration order. See srd002-sys R3.1, R3.2.
func OnTerminalResize(callback func(width int)) {
	panic("not implemented")
}
