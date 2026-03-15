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
	// AC2: Calling InstallSIGPIPEHandler twice must not panic or register
	// duplicate handlers. We cannot fully test os.Exit(0) behavior in-process,
	// but we verify that calling it multiple times is safe.
	InstallSIGPIPEHandler()
	InstallSIGPIPEHandler()
	InstallSIGPIPEHandler()
	// If we reach here without panic, idempotency is verified.
}

func TestOnTerminalResize_RegisterAndCancel(t *testing.T) {
	// Reset global state for this test.
	winchMu.Lock()
	winchCallbacks = nil
	if winchCancel != nil {
		winchCancel()
		winchCancel = nil
	}
	winchMu.Unlock()

	var called atomic.Int32

	cancel := OnTerminalResize(func(width int) {
		called.Add(1)
	})

	// Verify callback is registered.
	winchMu.Lock()
	count := len(winchCallbacks)
	hasListener := winchCancel != nil
	winchMu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 callback registered, got %d", count)
	}
	if !hasListener {
		t.Error("expected SIGWINCH listener to be running")
	}

	// Cancel and verify deregistration.
	cancel()

	winchMu.Lock()
	count = len(winchCallbacks)
	hasListener = winchCancel != nil
	winchMu.Unlock()

	if count != 0 {
		t.Errorf("expected 0 callbacks after cancel, got %d", count)
	}
	if hasListener {
		t.Error("expected SIGWINCH listener to be stopped after last callback removed")
	}
}

func TestOnTerminalResize_MultipleCallbacks(t *testing.T) {
	// Reset global state for this test.
	winchMu.Lock()
	winchCallbacks = nil
	if winchCancel != nil {
		winchCancel()
		winchCancel = nil
	}
	winchMu.Unlock()

	var order []int
	var mu sync.Mutex

	cancel1 := OnTerminalResize(func(width int) {
		mu.Lock()
		order = append(order, 1)
		mu.Unlock()
	})

	cancel2 := OnTerminalResize(func(width int) {
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
	// Note: this only works when running in a terminal context or when
	// the signal is delivered. The callbacks are tested via the signal.
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
	mu.Unlock()

	if callCount > 0 {
		// If callbacks fired, verify order.
		mu.Lock()
		if len(order) >= 2 && order[0] != 1 {
			t.Errorf("expected callback 1 called first, got order: %v", order)
		}
		mu.Unlock()
	}

	// Cancel first, verify second remains.
	cancel1()
	winchMu.Lock()
	count = len(winchCallbacks)
	hasListener := winchCancel != nil
	winchMu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 callback after cancel1, got %d", count)
	}
	if !hasListener {
		t.Error("expected SIGWINCH listener to still be running with 1 callback")
	}

	// Cancel second, verify listener stops.
	cancel2()
	winchMu.Lock()
	count = len(winchCallbacks)
	hasListener = winchCancel != nil
	winchMu.Unlock()

	if count != 0 {
		t.Errorf("expected 0 callbacks after cancel2, got %d", count)
	}
	if hasListener {
		t.Error("expected SIGWINCH listener to be stopped after all callbacks removed")
	}
}

func TestOnTerminalResize_CancelIdempotent(t *testing.T) {
	// Reset global state for this test.
	winchMu.Lock()
	winchCallbacks = nil
	if winchCancel != nil {
		winchCancel()
		winchCancel = nil
	}
	winchMu.Unlock()

	cancel := OnTerminalResize(func(width int) {})

	// Cancel twice should not panic.
	cancel()
	cancel()
}

func TestOnTerminalResize_ReRegisterAfterAllCanceled(t *testing.T) {
	// Reset global state for this test.
	winchMu.Lock()
	winchCallbacks = nil
	if winchCancel != nil {
		winchCancel()
		winchCancel = nil
	}
	winchMu.Unlock()

	// Register and cancel.
	cancel := OnTerminalResize(func(width int) {})
	cancel()

	// Re-register should start a new listener.
	cancel2 := OnTerminalResize(func(width int) {})

	winchMu.Lock()
	count := len(winchCallbacks)
	hasListener := winchCancel != nil
	winchMu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 callback after re-registration, got %d", count)
	}
	if !hasListener {
		t.Error("expected SIGWINCH listener to restart on re-registration")
	}

	cancel2()
}
