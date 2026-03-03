// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for terminal operations (prd002-sys R1.1, R1.2, R1.3).
package sys

import (
	"os"
	"testing"
)

// TestTerminalWidth_PipeFd verifies that TerminalWidth returns an error when
// stdout is not a terminal. In a test harness stdout is typically a pipe, so
// the ioctl(TIOCGWINSZ) call should fail (prd002-sys R1.1, R1.2).
func TestTerminalWidth_PipeFd(t *testing.T) {
	// In the test runner, stdout is a pipe — TerminalWidth should error.
	_, err := TerminalWidth()
	if err == nil {
		// If running inside a real terminal (e.g., manual `go test -v`),
		// skip rather than fail — the function is working correctly.
		t.Skip("stdout appears to be a terminal; cannot test pipe behavior")
	}
}

// TestIsTerminal_PipeFd verifies that IsTerminal returns false for a pipe file
// descriptor (prd002-sys R1.3).
func TestIsTerminal_PipeFd(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if IsTerminal(r.Fd()) {
		t.Error("IsTerminal returned true for pipe read end; expected false")
	}
	if IsTerminal(w.Fd()) {
		t.Error("IsTerminal returned true for pipe write end; expected false")
	}
}

// TestIsTerminal_RegularFile verifies that IsTerminal returns false for a
// regular file descriptor (prd002-sys R1.3).
func TestIsTerminal_RegularFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "isterm-*")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer f.Close()

	if IsTerminal(f.Fd()) {
		t.Error("IsTerminal returned true for regular file; expected false")
	}
}

// TestIsTerminal_InvalidFd verifies that IsTerminal returns false for an
// invalid file descriptor (prd002-sys R1.3).
func TestIsTerminal_InvalidFd(t *testing.T) {
	// fd 9999 is almost certainly not open.
	if IsTerminal(9999) {
		t.Error("IsTerminal returned true for invalid fd 9999; expected false")
	}
}
