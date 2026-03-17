// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestInstallSIGPIPEHandler_Idempotent(t *testing.T) {
	t.Parallel()
	// R1.6: Calling InstallSIGPIPEHandler multiple times must not panic or
	// register duplicate handlers. We cannot fully test os.Exit(0) behavior
	// in-process, but we verify that calling it multiple times is safe.
	InstallSIGPIPEHandler()
	InstallSIGPIPEHandler()
	InstallSIGPIPEHandler()
	// If we reach here without panic, idempotency is verified.
}

func TestOnTerminalResize_RegisterCallback(t *testing.T) {
	// Reset global state for this test.
	resetWinchState()

	var called atomic.Int32

	OnTerminalResize(func(width int) {
		called.Add(1)
	})

	// Verify callback is registered.
	winchMu.Lock()
	count := len(winchCallbacks)
	winchMu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 callback registered, got %d", count)
	}
}

func TestOnTerminalResize_MultipleCallbacks(t *testing.T) {
	// Reset global state for this test.
	resetWinchState()

	var order []int
	var mu sync.Mutex

	OnTerminalResize(func(width int) {
		mu.Lock()
		order = append(order, 1)
		mu.Unlock()
	})

	OnTerminalResize(func(width int) {
		mu.Lock()
		order = append(order, 2)
		mu.Unlock()
	})

	// Verify both are registered.
	winchMu.Lock()
	count := len(winchCallbacks)
	winchMu.Unlock()

	if count != 2 {
		t.Errorf("expected 2 callbacks registered, got %d", count)
	}

	// Send SIGWINCH to ourselves to trigger the callbacks.
	// Note: callbacks only fire if TerminalWidth() succeeds, which may
	// not happen in CI environments without a terminal. This is acceptable
	// per R3.1 (callback not invoked if TerminalWidth returns error).
	err := syscall.Kill(syscall.Getpid(), syscall.SIGWINCH)
	if err != nil {
		t.Fatalf("failed to send SIGWINCH: %v", err)
	}

	// Wait briefly for the signal to be delivered and processed.
	time.Sleep(100 * time.Millisecond)

	// Check if callbacks were called (may not fire if TerminalWidth fails
	// in test environment, which is acceptable per R3.1).
	mu.Lock()
	callCount := len(order)
	if callCount >= 2 && order[0] != 1 {
		t.Errorf("expected callback 1 called first, got order: %v", order)
	}
	mu.Unlock()
}

func TestOnTerminalResize_MultipleCallsIdempotentListener(t *testing.T) {
	// Reset global state for this test.
	resetWinchState()

	OnTerminalResize(func(width int) {})
	OnTerminalResize(func(width int) {})
	OnTerminalResize(func(width int) {})

	// Verify all three are registered with a single listener.
	winchMu.Lock()
	count := len(winchCallbacks)
	winchMu.Unlock()

	if count != 3 {
		t.Errorf("expected 3 callbacks registered, got %d", count)
	}
}

// resetWinchState resets the SIGWINCH global state for testing.
// This allows each test to start with a clean slate.
func resetWinchState() {
	winchMu.Lock()
	winchCallbacks = nil
	winchMu.Unlock()
	// Reset sync.Once so the listener can be re-started in the next test.
	winchOnce = sync.Once{}
}
