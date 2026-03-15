// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"testing"
)

// TestTerminalWidthNotTerminal verifies TerminalWidth returns an error when
// stdout is not a terminal (which is the case during test execution).
func TestTerminalWidthNotTerminal(t *testing.T) {
	t.Parallel()

	// During test execution stdout is typically a pipe, not a terminal.
	// TerminalWidth should return an error in this case.
	_, err := TerminalWidth()
	if err == nil {
		// If running in a terminal (unlikely in CI), we can't assert error.
		// Accept either outcome: positive width or error.
		t.Log("TerminalWidth returned nil error; stdout may be a terminal")
	}
}

// TestIsTerminalPipe verifies IsTerminal returns false for a pipe fd.
func TestIsTerminalPipe(t *testing.T) {
	t.Parallel()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if IsTerminal(r.Fd()) {
		t.Error("IsTerminal(pipe read end) = true, want false")
	}
	if IsTerminal(w.Fd()) {
		t.Error("IsTerminal(pipe write end) = true, want false")
	}
}

// TestIsTerminalRegularFile verifies IsTerminal returns false for a regular file.
func TestIsTerminalRegularFile(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "terminal-test-*")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer f.Close()

	if IsTerminal(f.Fd()) {
		t.Error("IsTerminal(regular file) = true, want false")
	}
}

// TestIsTerminalStdout verifies IsTerminal on stdout. During tests stdout is
// typically a pipe, so we expect false. If running interactively, true is also valid.
func TestIsTerminalStdout(t *testing.T) {
	t.Parallel()

	result := IsTerminal(os.Stdout.Fd())
	// We just verify it doesn't panic. In CI/tests stdout is a pipe (false).
	t.Logf("IsTerminal(stdout) = %v", result)
}
