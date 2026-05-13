// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

// TerminalWidth returns the current terminal column count by querying stdout.
// R1.1: returns an error when stdout is not a terminal.
func TerminalWidth() (int, error) {
	return 0, nil
}

// IsTerminal reports whether the file descriptor refers to a terminal.
// R1.3: returns false for pipes, regular files, and non-terminal descriptors.
func IsTerminal(fd uintptr) bool {
	return false
}
