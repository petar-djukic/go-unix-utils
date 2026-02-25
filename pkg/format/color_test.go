// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"bytes"
	"os"
	"testing"
)

func TestFileTypeColorAllTypes(t *testing.T) {
	// Force color enabled for these tests.
	SetColorEnabled(true)
	defer ResetColorEnabled()

	tests := []struct {
		name string
		mode os.FileMode
		want string
	}{
		{name: "directory", mode: os.ModeDir, want: "\033[34m"},
		{name: "symlink", mode: os.ModeSymlink, want: "\033[36m"},
		{name: "executable", mode: 0o755, want: "\033[32m"},
		{name: "block_device", mode: os.ModeDevice, want: "\033[33;1m"},
		{name: "char_device", mode: os.ModeDevice | os.ModeCharDevice, want: "\033[33;1m"},
		{name: "socket", mode: os.ModeSocket, want: "\033[35m"},
		{name: "pipe", mode: os.ModeNamedPipe, want: "\033[33m"},
		{name: "regular", mode: 0o644, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FileTypeColor(tt.mode)
			if got != tt.want {
				t.Errorf("FileTypeColor(%v) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestResetColorEnabled(t *testing.T) {
	SetColorEnabled(true)
	defer ResetColorEnabled()

	got := Reset()
	if got != "\033[0m" {
		t.Errorf("Reset() with color enabled = %q, want %q", got, "\033[0m")
	}
}

func TestResetColorDisabled(t *testing.T) {
	SetColorEnabled(false)
	defer ResetColorEnabled()

	got := Reset()
	if got != "" {
		t.Errorf("Reset() with color disabled = %q, want empty", got)
	}
}

func TestFileTypeColorDisabled(t *testing.T) {
	SetColorEnabled(false)
	defer ResetColorEnabled()

	// When color is disabled, all types return empty string.
	modes := []os.FileMode{
		os.ModeDir,
		os.ModeSymlink,
		0o755,
		os.ModeDevice,
		os.ModeSocket,
		os.ModeNamedPipe,
	}
	for _, mode := range modes {
		got := FileTypeColor(mode)
		if got != "" {
			t.Errorf("FileTypeColor(%v) with color disabled = %q, want empty", mode, got)
		}
	}
}

func TestColorEnabledPipe(t *testing.T) {
	// An os.Pipe is not a TTY, so ColorEnabled should return false.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer func() { _ = r.Close() }() // best-effort cleanup
	defer func() { _ = w.Close() }() // best-effort cleanup

	if ColorEnabled(w) {
		t.Error("ColorEnabled(pipe writer) = true, want false")
	}
}

func TestColorEnabledNonFile(t *testing.T) {
	// A bytes.Buffer is not an *os.File, so ColorEnabled should return false.
	var buf bytes.Buffer
	if ColorEnabled(&buf) {
		t.Error("ColorEnabled(bytes.Buffer) = true, want false")
	}
}
