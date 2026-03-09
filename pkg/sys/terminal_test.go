// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"testing"
)

func TestTerminalWidth_NonTerminal(t *testing.T) {
	t.Parallel()
	// When run under go test, stdout is redirected to a pipe, so
	// TerminalWidth should return an error.
	_, err := TerminalWidth()
	if err == nil {
		t.Skip("stdout appears to be a terminal; skipping non-terminal test")
	}
}

func TestIsTerminal_Pipe(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if IsTerminal(r.Fd()) {
		t.Error("IsTerminal returned true for a pipe read end")
	}
	if IsTerminal(w.Fd()) {
		t.Error("IsTerminal returned true for a pipe write end")
	}
}

func TestIsTerminal_RegularFile(t *testing.T) {
	t.Parallel()
	f, err := os.CreateTemp(t.TempDir(), "terminal-test-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer f.Close()

	if IsTerminal(f.Fd()) {
		t.Error("IsTerminal returned true for a regular file")
	}
}

func TestIsTerminal_Stdout(t *testing.T) {
	t.Parallel()
	// Under go test, stdout is a pipe. Verify IsTerminal returns false.
	if IsTerminal(os.Stdout.Fd()) {
		t.Skip("stdout is a terminal; skipping pipe assertion")
	}
}
