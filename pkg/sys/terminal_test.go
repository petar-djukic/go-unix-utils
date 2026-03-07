// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"testing"
)

func TestIsTerminalPipe(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	// AC3: A pipe fd is not a terminal.
	if IsTerminal(r.Fd()) {
		t.Error("IsTerminal(pipe read end) = true, want false")
	}
	if IsTerminal(w.Fd()) {
		t.Error("IsTerminal(pipe write end) = true, want false")
	}
}

func TestTerminalWidthNonTerminal(t *testing.T) {
	t.Parallel()
	// AC4: When not attached to a terminal, TerminalWidth returns an error.
	// In CI/test environments, stdout is typically not a terminal.
	if IsTerminal(os.Stdout.Fd()) {
		t.Skip("stdout is a terminal; cannot test non-terminal behavior")
	}

	_, err := TerminalWidth()
	if err == nil {
		t.Error("TerminalWidth() on non-terminal stdout: expected error, got nil")
	}
}

func TestIsTerminalRegularFile(t *testing.T) {
	t.Parallel()
	f, err := os.CreateTemp(t.TempDir(), "term")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer f.Close()

	if IsTerminal(f.Fd()) {
		t.Error("IsTerminal(regular file) = true, want false")
	}
}
