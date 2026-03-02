// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for terminal size query functions.
//
// Implements: prd002-sys R1.1, R1.2
package sys

import (
	"os"
	"testing"
)

// --- TerminalSize non-TTY error path (prd002-sys R1.1, R1.2) ---

func TestTerminalSize_NonTTY(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	origStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	cols, rows, err := TerminalSize()
	if err == nil {
		t.Errorf("TerminalSize() on pipe: expected error, got cols=%d rows=%d", cols, rows)
	}
}
