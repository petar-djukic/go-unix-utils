// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin || linux

// Tests for prd002-sys R1.1, R1.2: TerminalWidth non-terminal FD error paths.

package sys_test

import (
	"os"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// TestTerminalWidth_PipeReadEnd verifies that TerminalWidth returns a non-nil
// error when passed the read end of an os.Pipe file descriptor.
// R1.1: non-terminal FD must be rejected.
func TestTerminalWidth_PipeReadEnd(t *testing.T) {
	t.Parallel()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	defer r.Close()
	defer w.Close()

	_, err = sys.TerminalWidth(int(r.Fd()))
	if err == nil {
		t.Error("TerminalWidth(pipe read end) returned nil error, want non-nil")
	}
}

// TestTerminalWidth_PipeWriteEnd verifies that TerminalWidth returns a non-nil
// error when passed the write end of an os.Pipe file descriptor.
// R1.1: second non-terminal FD exercised.
func TestTerminalWidth_PipeWriteEnd(t *testing.T) {
	t.Parallel()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	defer r.Close()
	defer w.Close()

	_, err = sys.TerminalWidth(int(w.Fd()))
	if err == nil {
		t.Error("TerminalWidth(pipe write end) returned nil error, want non-nil")
	}
}

// TestTerminalWidth_RegularFile verifies that TerminalWidth returns a non-nil
// error when passed a regular file descriptor opened via os.CreateTemp.
// R1.1: file-backed FDs are not terminals.
func TestTerminalWidth_RegularFile(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "terminal-test-*")
	if err != nil {
		t.Fatalf("os.CreateTemp() failed: %v", err)
	}
	defer f.Close()

	_, err = sys.TerminalWidth(int(f.Fd()))
	if err == nil {
		t.Error("TerminalWidth(regular file) returned nil error, want non-nil")
	}
}

// TestTerminalWidth_NoPanic verifies that TerminalWidth does not panic for any
// valid non-terminal file descriptor. Each subtest exercises a different FD
// type and confirms clean error return without crashing.
// R1.1: robust error handling for non-terminal FDs.
func TestTerminalWidth_NoPanic(t *testing.T) {
	t.Parallel()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	defer r.Close()
	defer w.Close()

	f, err := os.CreateTemp(t.TempDir(), "terminal-nopanic-*")
	if err != nil {
		t.Fatalf("os.CreateTemp() failed: %v", err)
	}
	defer f.Close()

	tests := []struct {
		name string
		fd   int
	}{
		{"pipe_read", int(r.Fd())},
		{"pipe_write", int(w.Fd())},
		{"regular_file", int(f.Fd())},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// If TerminalWidth panics, the test framework will catch it and
			// report a failure. We only need to call the function.
			_, err := sys.TerminalWidth(tc.fd)
			if err == nil {
				t.Errorf("TerminalWidth(%s) returned nil error, want non-nil", tc.name)
			}
		})
	}
}
