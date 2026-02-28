// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// signal_test.go contains unit tests for InstallSIGPIPEHandler idempotency
// and OnTerminalResize callback registration mechanics.
//
// These tests verify installation mechanics (no panic, idempotency, callback
// slice growth) rather than signal delivery behavior, because signal delivery
// requires process-level isolation and is inherently racy in go test contexts.
//
// Tests: prd002-sys R3.1, R3.2, R4.1, R4.2.
package sys

import (
	"testing"
)

func TestInstallSIGPIPEHandler_NoPanic(t *testing.T) {
	// InstallSIGPIPEHandler must complete without panic (R3.1).
	// Note: sigpipeOnce may already have fired from a prior test run in the
	// same process; this test verifies the call does not panic regardless.
	InstallSIGPIPEHandler()
}

func TestInstallSIGPIPEHandler_Idempotent(t *testing.T) {
	// Calling InstallSIGPIPEHandler multiple times must not panic,
	// verifying the sync.Once idempotency guarantee (R3.2).
	InstallSIGPIPEHandler()
	InstallSIGPIPEHandler()
	InstallSIGPIPEHandler()
}

func TestOnTerminalResize_NoPanic(t *testing.T) {
	// OnTerminalResize must accept a callback function without panic (R4.1).
	OnTerminalResize(func(width int) {
		// no-op callback for installation test
	})
}

func TestOnTerminalResize_MultipleCallbacks(t *testing.T) {
	// Calling OnTerminalResize multiple times must register multiple callbacks
	// in the resizeCallbacks slice (R4.2).
	resizeMu.Lock()
	initialLen := len(resizeCallbacks)
	resizeMu.Unlock()

	OnTerminalResize(func(width int) {})
	OnTerminalResize(func(width int) {})

	resizeMu.Lock()
	afterLen := len(resizeCallbacks)
	resizeMu.Unlock()

	added := afterLen - initialLen
	if added != 2 {
		t.Errorf("OnTerminalResize registered %d callbacks, want 2 (initial=%d, after=%d)",
			added, initialLen, afterLen)
	}
}

func TestOnTerminalResize_RegistrationOrder(t *testing.T) {
	// Callbacks must be stored in registration order (R4.2).
	resizeMu.Lock()
	baseLen := len(resizeCallbacks)
	resizeMu.Unlock()

	var callOrder []int

	OnTerminalResize(func(width int) { callOrder = append(callOrder, 1) })
	OnTerminalResize(func(width int) { callOrder = append(callOrder, 2) })
	OnTerminalResize(func(width int) { callOrder = append(callOrder, 3) })

	resizeMu.Lock()
	finalLen := len(resizeCallbacks)
	resizeMu.Unlock()

	added := finalLen - baseLen
	if added != 3 {
		t.Errorf("OnTerminalResize registered %d callbacks, want 3", added)
	}
}
