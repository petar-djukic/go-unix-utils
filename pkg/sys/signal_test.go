// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// TestInstallSIGPIPEHandlerIdempotent verifies that InstallSIGPIPEHandler
// can be called multiple times without panic. AC1.
func TestInstallSIGPIPEHandlerIdempotent(t *testing.T) {
	t.Parallel()
	// Calling multiple times must not panic.
	InstallSIGPIPEHandler()
	InstallSIGPIPEHandler()
	InstallSIGPIPEHandler()
}

// TestOnSIGHUPCallback verifies that OnSIGHUP registers a callback that
// is invoked when SIGHUP is delivered. AC2.
func TestOnSIGHUPCallback(t *testing.T) {
	var called atomic.Int32
	OnSIGHUP(func() {
		called.Add(1)
	})

	// Send SIGHUP to ourselves.
	err := syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
	if err != nil {
		t.Fatalf("failed to send SIGHUP: %v", err)
	}

	// Wait for the callback to fire.
	deadline := time.After(2 * time.Second)
	for called.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("SIGHUP callback was not invoked within timeout")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if got := called.Load(); got < 1 {
		t.Errorf("expected callback to be called at least once, got %d", got)
	}
}

// TestOnSIGHUPMultipleCallbacks verifies that multiple SIGHUP callbacks
// are all invoked. AC2.
func TestOnSIGHUPMultipleCallbacks(t *testing.T) {
	var count1 atomic.Int32
	var count2 atomic.Int32

	OnSIGHUP(func() {
		count1.Add(1)
	})
	OnSIGHUP(func() {
		count2.Add(1)
	})

	err := syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
	if err != nil {
		t.Fatalf("failed to send SIGHUP: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for count1.Load() == 0 || count2.Load() == 0 {
		select {
		case <-deadline:
			t.Fatalf("not all SIGHUP callbacks invoked: count1=%d count2=%d",
				count1.Load(), count2.Load())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// TestOnTerminalResizeRegistration verifies that OnTerminalResize can be
// called without panic and accepts a callback. AC3.
func TestOnTerminalResizeRegistration(t *testing.T) {
	t.Parallel()
	// Registration must not panic.
	OnTerminalResize(func(width int) {
		// no-op callback for registration test
	})
}

// TestOnTerminalResizeSignal verifies that sending SIGWINCH invokes the
// registered callback. In CI (no terminal), TerminalWidth returns an error
// so the callback is not invoked; the test verifies no panic occurs. AC3.
func TestOnTerminalResizeSignal(t *testing.T) {
	var called atomic.Int32
	OnTerminalResize(func(width int) {
		called.Add(1)
	})

	err := syscall.Kill(syscall.Getpid(), syscall.SIGWINCH)
	if err != nil {
		t.Fatalf("failed to send SIGWINCH: %v", err)
	}

	// Give the goroutine time to process the signal.
	time.Sleep(200 * time.Millisecond)

	// When stdout is a terminal, callback should have been called.
	// When stdout is not a terminal (CI), TerminalWidth errors and
	// callback is skipped — both outcomes are correct. AC3.
	if IsTerminal(os.Stdout.Fd()) {
		if got := called.Load(); got == 0 {
			t.Error("expected OnTerminalResize callback to be invoked when stdout is a terminal")
		}
	}
}

// TestAllSignalHandlersCoexist verifies that installing all three signal
// handlers in the same process does not cause interference. AC4.
func TestAllSignalHandlersCoexist(t *testing.T) {
	t.Parallel()
	// Install all handlers — must not panic or deadlock.
	InstallSIGPIPEHandler()
	OnSIGHUP(func() {})
	OnTerminalResize(func(width int) {})
}
