// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys_test

import (
	"os"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func TestIsTerminal_Pipe(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if sys.IsTerminal(r.Fd()) {
		t.Error("IsTerminal(pipe read fd) = true, want false")
	}
	if sys.IsTerminal(w.Fd()) {
		t.Error("IsTerminal(pipe write fd) = true, want false")
	}
}

func TestTerminalWidth_ReturnsOrErrors(t *testing.T) {
	t.Parallel()
	width, err := sys.TerminalWidth()
	if err != nil {
		// In CI or piped environments, stdout is not a TTY — this is expected.
		t.Logf("TerminalWidth returned error (expected in non-TTY environment): %v", err)
		return
	}
	if width <= 0 {
		t.Errorf("TerminalWidth() = %d, want > 0", width)
	}
}

func TestOnTerminalResize_Registers(t *testing.T) {
	t.Parallel()
	// Verify OnTerminalResize is callable without panic. Actual SIGWINCH
	// testing requires a real terminal resize event and is out of scope
	// for automated tests (see rel00.0-uc003-sys out_of_scope).
	called := false
	sys.OnTerminalResize(func(width int) {
		called = true
	})
	// We cannot trigger SIGWINCH reliably in a test, but the callback
	// registration must not panic.
	_ = called
}
