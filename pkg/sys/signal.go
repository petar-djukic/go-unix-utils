// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

// OnTerminalResize registers a callback invoked with the new terminal width
// when a SIGWINCH signal is received.
func OnTerminalResize(callback func(width int)) {
}
