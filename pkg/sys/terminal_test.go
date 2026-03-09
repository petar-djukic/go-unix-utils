// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"testing"
)

// TestIsTerminal_Pipe verifies that IsTerminal returns false for a pipe fd.
// AC5: IsTerminal returns false for pipes/files.
func TestIsTerminal_Pipe(t *testing.T) {
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

// TestIsTerminal_File verifies that IsTerminal returns false for a regular file fd.
func TestIsTerminal_File(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "termtest")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer f.Close()

	if IsTerminal(f.Fd()) {
		t.Error("IsTerminal(regular file) = true, want false")
	}
}

// TestTerminalWidth_NonTTY verifies that TerminalWidth returns an error when
// stdout is not a terminal (which is the case in test processes piped to go test).
// AC4: TerminalWidth returns an error when stdout is not a terminal.
func TestTerminalWidth_NonTTY(t *testing.T) {
	t.Parallel()

	// In test processes, stdout is typically a pipe, not a terminal.
	// If stdout happens to be a terminal, skip this test.
	if IsTerminal(os.Stdout.Fd()) {
		t.Skip("stdout is a terminal; cannot test non-TTY path")
	}

	_, err := TerminalWidth()
	if err == nil {
		t.Error("TerminalWidth() returned nil error when stdout is not a terminal")
	}
}
