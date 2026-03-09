// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"testing"
)

func TestIsTerminalPipe(t *testing.T) {
	t.Parallel()

	// AC4: pipes are not terminals.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if IsTerminal(r.Fd()) {
		t.Error("IsTerminal on pipe read end: expected false")
	}
	if IsTerminal(w.Fd()) {
		t.Error("IsTerminal on pipe write end: expected false")
	}
}

func TestIsTerminalRegularFile(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "test")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer f.Close()

	if IsTerminal(f.Fd()) {
		t.Error("IsTerminal on regular file: expected false")
	}
}

func TestTerminalWidthNonTerminal(t *testing.T) {
	t.Parallel()

	// AC3: TerminalWidth returns an error on a non-terminal fd.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = origStdout
		r.Close()
		w.Close()
	}()

	_, err = TerminalWidth()
	if err == nil {
		t.Error("TerminalWidth with piped stdout: expected error, got nil")
	}
}
