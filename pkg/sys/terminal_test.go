// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// terminal_test.go contains unit tests for TerminalSize, TerminalWidth,
// DefaultTerminalSize, and IsTerminal, verifying error behavior for
// non-terminal file descriptors and fallback defaults.
//
// Tests: prd002-sys R1.1, R1.2, R1.3.
package sys

import (
	"os"
	"testing"
)

func TestTerminalSize_NonTerminalFD(t *testing.T) {
	// TerminalSize must return an error for non-terminal file descriptors (R1.1).
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	defer r.Close()
	defer w.Close()

	tests := []struct {
		name string
		fd   uintptr
	}{
		{name: "pipe_read_end", fd: r.Fd()},
		{name: "pipe_write_end", fd: w.Fd()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols, rows, err := TerminalSize(tt.fd)
			if err == nil {
				t.Errorf("TerminalSize(%d) = (%d, %d, nil), want error for non-terminal fd",
					tt.fd, cols, rows)
			}
		})
	}
}

func TestTerminalWidth_NonTerminal(t *testing.T) {
	// TerminalWidth queries stdout, which is not a terminal in go test (R1.1).
	// It should return an error when stdout is a pipe (as in test environments).
	_, err := TerminalWidth()
	if err == nil {
		// In CI/test environments stdout is typically not a terminal.
		// If this runs in a real terminal, skip rather than fail.
		t.Skip("stdout appears to be a terminal; skipping non-terminal test")
	}
}

func TestDefaultTerminalSize(t *testing.T) {
	// DefaultTerminalSize must return (80, 24) when stdout is not a terminal,
	// verifying the defaultCols and defaultRows fallback constants (R1.1).
	cols, rows := DefaultTerminalSize()

	// In go test, stdout is typically a pipe (not a terminal), so we expect
	// the fallback values. If stdout is a terminal, we accept any positive values.
	_, _, err := TerminalSize(os.Stdout.Fd())
	if err != nil {
		// stdout is not a terminal — expect fallback defaults.
		if cols != defaultCols {
			t.Errorf("DefaultTerminalSize() cols = %d, want %d (fallback)", cols, defaultCols)
		}
		if rows != defaultRows {
			t.Errorf("DefaultTerminalSize() rows = %d, want %d (fallback)", rows, defaultRows)
		}
	} else {
		// stdout is a real terminal — values must be positive.
		if cols <= 0 {
			t.Errorf("DefaultTerminalSize() cols = %d, want positive", cols)
		}
		if rows <= 0 {
			t.Errorf("DefaultTerminalSize() rows = %d, want positive", rows)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	// IsTerminal must return false for pipe file descriptors (R1.3).
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	defer r.Close()
	defer w.Close()

	tests := []struct {
		name string
		fd   uintptr
	}{
		{name: "pipe_read_end", fd: r.Fd()},
		{name: "pipe_write_end", fd: w.Fd()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if IsTerminal(tt.fd) {
				t.Errorf("IsTerminal(%d) = true, want false for pipe fd", tt.fd)
			}
		})
	}
}

func TestIsTerminal_RegularFile(t *testing.T) {
	// IsTerminal must return false for regular files (R1.3).
	f, err := os.CreateTemp(t.TempDir(), "isterm-test-*")
	if err != nil {
		t.Fatalf("os.CreateTemp() failed: %v", err)
	}
	defer f.Close()

	if IsTerminal(f.Fd()) {
		t.Errorf("IsTerminal(regular file fd) = true, want false")
	}
}
