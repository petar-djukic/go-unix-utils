// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"testing"
)

// TestTerminalWidth_NonTerminal verifies that TerminalWidth returns an error
// when stdout is not a terminal (as in test runners and CI).
func TestTerminalWidth_NonTerminal(t *testing.T) {
	t.Parallel()
	// In a test runner, stdout is typically a pipe, not a terminal.
	// We cannot reliably test the positive case without a pty.
	if !IsTerminal(os.Stdout.Fd()) {
		_, err := TerminalWidth()
		if err == nil {
			t.Error("expected error from TerminalWidth when stdout is not a terminal")
		}
	}
}

// TestIsTerminal_Pipe verifies that IsTerminal returns false for a pipe fd.
func TestIsTerminal_Pipe(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if IsTerminal(r.Fd()) {
		t.Error("expected IsTerminal to return false for pipe read end")
	}
	if IsTerminal(w.Fd()) {
		t.Error("expected IsTerminal to return false for pipe write end")
	}
}

// TestIsTerminal_RegularFile verifies that IsTerminal returns false for a
// regular file descriptor.
func TestIsTerminal_RegularFile(t *testing.T) {
	t.Parallel()
	f, err := os.CreateTemp(t.TempDir(), "isterm")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer f.Close()

	if IsTerminal(f.Fd()) {
		t.Error("expected IsTerminal to return false for regular file")
	}
}

// TestIsTerminal_Stdout checks that IsTerminal on stdout matches the
// environment (false in test runners, true in interactive terminals).
func TestIsTerminal_Stdout(t *testing.T) {
	t.Parallel()
	// Just verify it does not panic; the result depends on the environment.
	_ = IsTerminal(os.Stdout.Fd())
}
