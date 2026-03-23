// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R1.3: TerminalWidth and IsTerminal signatures.
package sys

// TerminalWidth returns the width of the controlling terminal in columns.
//
// R1.3: stub implementation — panics until the execution logic is implemented.
func TerminalWidth() (int, error) {
	panic("not implemented")
}

// IsTerminal reports whether the given file descriptor refers to a terminal.
//
// R1.3: stub implementation — panics until the execution logic is implemented.
func IsTerminal(fd uintptr) bool {
	panic("not implemented")
}
