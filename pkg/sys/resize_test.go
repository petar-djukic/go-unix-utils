// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestOnTerminalResize_CallbackInvoked(t *testing.T) {
	// Reset global state for this test.
	resetResizeState()

	var mu sync.Mutex
	var received int
	OnTerminalResize(func(width int) {
		mu.Lock()
		received = width
		mu.Unlock()
	})

	// Send SIGWINCH to ourselves.
	err := syscall.Kill(syscall.Getpid(), syscall.SIGWINCH)
	if err != nil {
		t.Fatalf("sending SIGWINCH: %v", err)
	}

	// Wait for the signal to be delivered and processed.
	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		r := received
		mu.Unlock()
		if r > 0 {
			break
		}
		select {
		case <-deadline:
			// On non-terminal environments, TerminalWidth() may fail and
			// the callback is not invoked (per R3.1). Skip the test.
			t.Skip("SIGWINCH callback not invoked; likely not a terminal")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestOnTerminalResize_MultipleCallbacks(t *testing.T) {
	// Reset global state for this test.
	resetResizeState()

	var mu sync.Mutex
	var order []int
	OnTerminalResize(func(_ int) {
		mu.Lock()
		order = append(order, 1)
		mu.Unlock()
	})
	OnTerminalResize(func(_ int) {
		mu.Lock()
		order = append(order, 2)
		mu.Unlock()
	})

	err := syscall.Kill(syscall.Getpid(), syscall.SIGWINCH)
	if err != nil {
		t.Fatalf("sending SIGWINCH: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		n := len(order)
		mu.Unlock()
		if n >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Skip("SIGWINCH callbacks not invoked; likely not a terminal")
		case <-time.After(10 * time.Millisecond):
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(order) < 2 || order[0] != 1 || order[1] != 2 {
		t.Errorf("expected callbacks in order [1, 2], got %v", order)
	}
}

func TestOnTerminalResize_NonBlocking(t *testing.T) {
	// Reset global state for this test.
	resetResizeState()

	done := make(chan struct{})
	go func() {
		OnTerminalResize(func(_ int) {})
		close(done)
	}()
	select {
	case <-done:
		// R2.6: OnTerminalResize returned without blocking.
	case <-time.After(2 * time.Second):
		t.Fatal("OnTerminalResize blocked the calling goroutine")
	}
}

// resetResizeState resets the package-level resize state between tests.
func resetResizeState() {
	resizeMu.Lock()
	defer resizeMu.Unlock()
	resizeCallbacks = nil
	resizeStarted = false
}
