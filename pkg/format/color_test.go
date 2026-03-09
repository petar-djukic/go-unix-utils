// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"bytes"
	"os"
	"testing"
)

func TestFileTypeColor(t *testing.T) {
	t.Parallel()

	// Force color on so we can test the return values. AC2.
	SetColorEnabled(true)
	t.Cleanup(ResetColorEnabled)

	tests := []struct {
		name string
		mode os.FileMode
		want string
	}{
		{"directory", os.ModeDir, "\033[34m"},
		{"symlink", os.ModeSymlink, "\033[36m"},
		{"executable", 0o755, "\033[32m"},
		{"block device", os.ModeDevice, "\033[33;1m"},
		{"char device", os.ModeDevice | os.ModeCharDevice, "\033[33;1m"},
		{"socket", os.ModeSocket, "\033[35m"},
		{"pipe", os.ModeNamedPipe, "\033[33m"},
		{"regular file", 0o644, ""},
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

func TestReset(t *testing.T) {
	t.Parallel()

	SetColorEnabled(true)
	t.Cleanup(ResetColorEnabled)

	got := Reset()
	if got != "\033[0m" {
		t.Errorf("Reset() = %q, want %q", got, "\033[0m")
	}
}

func TestColorEnabledNonTerminal(t *testing.T) {
	t.Parallel()

	// AC3: bytes.Buffer is not a terminal.
	var buf bytes.Buffer
	if ColorEnabled(&buf) {
		t.Error("ColorEnabled(bytes.Buffer) = true, want false")
	}
}

func TestColorEnabledPipe(t *testing.T) {
	t.Parallel()

	// AC3: pipe is not a terminal.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if ColorEnabled(w) {
		t.Error("ColorEnabled(pipe writer) = true, want false")
	}
}

func TestSetColorEnabled(t *testing.T) {
	// AC5: override forces color on/off regardless of writer type.

	// Force on: FileTypeColor returns ANSI even though stdout might be a pipe.
	SetColorEnabled(true)
	got := FileTypeColor(os.ModeDir)
	if got != "\033[34m" {
		t.Errorf("with SetColorEnabled(true), FileTypeColor(Dir) = %q, want blue", got)
	}
	gotReset := Reset()
	if gotReset != "\033[0m" {
		t.Errorf("with SetColorEnabled(true), Reset() = %q, want reset", gotReset)
	}

	// Force off: returns empty even for directory.
	SetColorEnabled(false)
	got = FileTypeColor(os.ModeDir)
	if got != "" {
		t.Errorf("with SetColorEnabled(false), FileTypeColor(Dir) = %q, want empty", got)
	}
	gotReset = Reset()
	if gotReset != "" {
		t.Errorf("with SetColorEnabled(false), Reset() = %q, want empty", gotReset)
	}

	// Reset: reverts to auto-detect.
	ResetColorEnabled()
}
