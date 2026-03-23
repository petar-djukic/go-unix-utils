// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R3.1–R3.2: OnTerminalResize signature.
package sys

// OnTerminalResize registers a callback that is invoked whenever the terminal
// is resized (SIGWINCH). The callback receives the new terminal width.
//
// R3.1: stub implementation — panics until the execution logic is implemented.
func OnTerminalResize(callback func(width int)) {
	panic("not implemented")
}
