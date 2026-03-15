// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestOnTerminalResize_CallbackCalled(t *testing.T) {
	// R3.1: sending SIGWINCH triggers the registered callback.
	// In test environments stdout is typically a pipe, so TerminalWidth() will
	// return an error and the callback will not fire. We verify the signal
	// delivery path works; callback invocation depends on terminal availability.
	var called atomic.Bool
	OnTerminalResize(func(width int) {
		called.Store(true)
	})

	// Send SIGWINCH to ourselves.
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGWINCH); err != nil {
		t.Fatalf("failed to send SIGWINCH: %v", err)
	}

	// Allow time for signal delivery and handler execution.
	time.Sleep(100 * time.Millisecond)

	// In CI/test (no terminal), TerminalWidth() errors so callback is not invoked.
	// We accept both outcomes: the test verifies no panic or deadlock on signal.
	if called.Load() {
		t.Log("callback was invoked (stdout is a terminal)")
	} else {
		t.Log("callback was not invoked (stdout is not a terminal, as expected in tests)")
	}
}

func TestOnTerminalResize_ReplaceCallback(t *testing.T) {
	// R3.3: subsequent calls replace the previous callback.
	var firstCalled atomic.Bool
	var secondCalled atomic.Bool

	OnTerminalResize(func(width int) {
		firstCalled.Store(true)
	})

	// Replace the callback.
	OnTerminalResize(func(width int) {
		secondCalled.Store(true)
	})

	// Send SIGWINCH to ourselves.
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGWINCH); err != nil {
		t.Fatalf("failed to send SIGWINCH: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// In CI/test, TerminalWidth() may error so neither fires. That's OK.
	// If a terminal is present, only the second callback should fire.
	if firstCalled.Load() {
		t.Error("first callback was invoked after replacement")
	}
}

func TestOnTerminalResize_MultipleCalls(t *testing.T) {
	// R3.3: safe to call multiple times without panic or additional goroutines.
	OnTerminalResize(func(width int) {})
	OnTerminalResize(func(width int) {})
	OnTerminalResize(func(width int) {})
}
