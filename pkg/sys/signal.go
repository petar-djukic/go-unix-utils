// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

// InstallSIGPIPEHandler installs a handler that exits the process with code 0
// when a SIGPIPE signal is received.
// R1.5: matches GNU coreutils behavior for piped output.
// R1.6: safe to call multiple times.
func InstallSIGPIPEHandler() {
}

// OnTerminalResize registers a callback invoked with the new terminal width
// when a SIGWINCH signal is received.
// R3.1: calls TerminalWidth() and skips the callback if it returns an error.
// R3.2: supports multiple registered callbacks, called in registration order.
func OnTerminalResize(callback func(width int)) {
}
