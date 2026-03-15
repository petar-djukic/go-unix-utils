// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"testing"
)

func TestTerminalWidth_NotATTY(t *testing.T) {
	t.Parallel()

	// When stdout is not a terminal (as in go test), TerminalWidth returns an error.
	_, err := TerminalWidth()
	if err == nil {
		// In CI or go test, stdout is typically a pipe, so we expect an error.
		// If this test runs in a real terminal, TerminalWidth succeeds, which is also valid.
		t.Log("TerminalWidth succeeded — stdout appears to be a terminal")
	}
}

func TestIsTerminal_Pipe(t *testing.T) {
	t.Parallel()

	// A pipe fd is not a terminal.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if IsTerminal(r.Fd()) {
		t.Error("IsTerminal returned true for pipe read end")
	}
	if IsTerminal(w.Fd()) {
		t.Error("IsTerminal returned true for pipe write end")
	}
}

func TestIsTerminal_RegularFile(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "isterm-test-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer f.Close()

	if IsTerminal(f.Fd()) {
		t.Error("IsTerminal returned true for regular file")
	}
}
