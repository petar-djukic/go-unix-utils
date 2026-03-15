// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestReset(t *testing.T) {
	t.Parallel()
	got := Reset()
	want := "\033[0m"
	if got != want {
		t.Errorf("Reset() = %q, want %q", got, want)
	}
}

func TestFileTypeColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode os.FileMode
		want string
	}{
		{"regular file", 0o644, ""},
		{"directory", os.ModeDir | 0o755, "\033[01;34m"},
		{"symlink", os.ModeSymlink | 0o777, "\033[01;36m"},
		{"executable", 0o755, "\033[01;32m"},
		{"named pipe", os.ModeNamedPipe | 0o644, "\033[33m"},
		{"socket", os.ModeSocket | 0o755, "\033[01;35m"},
		{"block device", os.ModeDevice | 0o660, "\033[01;33m"},
		{"char device", os.ModeDevice | os.ModeCharDevice | 0o660, "\033[01;33m"},
		{"setuid", os.ModeSetuid | 0o4755, "\033[37;41m"},
		{"setgid", os.ModeSetgid | 0o2755, "\033[30;43m"},
		{"sticky directory", os.ModeDir | os.ModeSticky | 0o755, "\033[37;44m"},
		{"other-writable+sticky directory", os.ModeDir | os.ModeSticky | 0o1777, "\033[30;42m"},
		{"other-writable directory (no sticky)", os.ModeDir | 0o777, "\033[34;42m"},
		{"setuid takes priority over setgid", os.ModeSetuid | os.ModeSetgid | 0o6755, "\033[37;41m"},
		{"executable no other bits", 0o100, "\033[01;32m"},
		{"regular no execute", 0o600, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FileTypeColor(tc.mode)
			if got != tc.want {
				t.Errorf("FileTypeColor(%v) = %q, want %q", tc.mode, got, tc.want)
			}
		})
	}
}

func TestColorEnabledPipe(t *testing.T) {
	t.Parallel()

	// A pipe is not a terminal, so ColorEnabled should return false.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	ResetColorEnabled()
	defer ResetColorEnabled()

	if ColorEnabled(w) {
		t.Error("ColorEnabled(pipe) = true, want false")
	}
}

func TestColorEnabledNonFile(t *testing.T) {
	t.Parallel()

	// A bytes.Buffer is not an *os.File, so ColorEnabled should return false.
	var buf bytes.Buffer
	ResetColorEnabled()
	defer ResetColorEnabled()

	if ColorEnabled(&buf) {
		t.Error("ColorEnabled(bytes.Buffer) = true, want false")
	}
}

func TestColorEnabledNilWriter(t *testing.T) {
	t.Parallel()

	// io.Discard is not an *os.File.
	ResetColorEnabled()
	defer ResetColorEnabled()

	if ColorEnabled(io.Discard) {
		t.Error("ColorEnabled(io.Discard) = true, want false")
	}
}

func TestSetColorEnabledTrue(t *testing.T) {
	// Not parallel: mutates package-level state.
	defer ResetColorEnabled()

	SetColorEnabled(true)

	// Even a pipe should report color enabled when forced.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if !ColorEnabled(w) {
		t.Error("ColorEnabled(pipe) with SetColorEnabled(true) = false, want true")
	}

	// Non-file writer should also report true.
	var buf bytes.Buffer
	if !ColorEnabled(&buf) {
		t.Error("ColorEnabled(buffer) with SetColorEnabled(true) = false, want true")
	}
}

func TestSetColorEnabledFalse(t *testing.T) {
	// Not parallel: mutates package-level state.
	defer ResetColorEnabled()

	SetColorEnabled(false)

	// Even stdout should report color disabled when forced off.
	if ColorEnabled(os.Stdout) {
		t.Error("ColorEnabled(stdout) with SetColorEnabled(false) = true, want false")
	}
}

func TestResetColorEnabled(t *testing.T) {
	// Not parallel: mutates package-level state.
	defer ResetColorEnabled()

	// Force on, then reset — pipe should return false again.
	SetColorEnabled(true)
	ResetColorEnabled()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if ColorEnabled(w) {
		t.Error("ColorEnabled(pipe) after ResetColorEnabled = true, want false")
	}
}
