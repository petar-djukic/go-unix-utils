// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R1.4: InstallSIGPIPEHandler and OnTerminalResize signatures.
package sys

// InstallSIGPIPEHandler sets up a SIGPIPE signal handler so that utilities
// writing to stdout exit cleanly when piped to a consumer that closes stdin
// early, matching GNU coreutils behavior.
//
// R1.4: stub implementation — panics until the execution logic is implemented.
func InstallSIGPIPEHandler() {
	panic("not implemented")
}

// OnTerminalResize registers a callback that is invoked whenever the terminal
// is resized (SIGWINCH). The callback receives the new terminal width.
//
// R1.4: stub implementation — panics until the execution logic is implemented.
func OnTerminalResize(callback func(width int)) {
	panic("not implemented")
}
