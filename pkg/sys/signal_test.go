// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for SIGPIPE and SIGWINCH signal handling (prd002-sys R1.5, R1.6, R3.1, R3.2).
package sys

import (
	"testing"
)

// TestInstallSIGPIPEHandler_NoPanic verifies that InstallSIGPIPEHandler can be
// called without panicking (prd002-sys R1.5).
func TestInstallSIGPIPEHandler_NoPanic(t *testing.T) {
	// Calling InstallSIGPIPEHandler should not panic.
	InstallSIGPIPEHandler()
}

// TestInstallSIGPIPEHandler_MultipleCallsSafe verifies that
// InstallSIGPIPEHandler is safe to call multiple times without panic or error
// (prd002-sys R1.6).
func TestInstallSIGPIPEHandler_MultipleCallsSafe(t *testing.T) {
	for i := 0; i < 3; i++ {
		InstallSIGPIPEHandler()
	}
}

// TestOnTerminalResize_NoPanic verifies that OnTerminalResize can be called
// with a callback without panicking (prd002-sys R3.1).
func TestOnTerminalResize_NoPanic(t *testing.T) {
	OnTerminalResize(func(width int) {
		// no-op callback for registration test
	})
}

// TestOnTerminalResize_MultipleRegistrations verifies that multiple callbacks
// can be registered without panic. The callback list grows with each call
// (prd002-sys R3.2).
func TestOnTerminalResize_MultipleRegistrations(t *testing.T) {
	for i := 0; i < 3; i++ {
		OnTerminalResize(func(width int) {
			// no-op callback
		})
	}

	// Verify that the global callback list has grown. We access the package-level
	// resizeCallbacks slice (white-box test per design decision D3).
	resizeMu.Lock()
	count := len(resizeCallbacks)
	resizeMu.Unlock()

	// We cannot know the exact count because other tests may have registered
	// callbacks, but we should have at least the 3 we just added plus the 1
	// from TestOnTerminalResize_NoPanic.
	if count < 4 {
		t.Errorf("resizeCallbacks has %d entries; expected at least 4", count)
	}
}
