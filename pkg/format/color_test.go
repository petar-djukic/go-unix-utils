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

	// AC2: distinct ANSI sequences for each file type.
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
		{"named pipe", os.ModeNamedPipe, "\033[33m"},
		{"regular file", 0o644, "\033[0m"},
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
	want := "\033[0m"
	got := Reset()
	if got != want {
		t.Errorf("Reset() = %q, want %q", got, want)
	}
}

func TestColorEnabled(t *testing.T) {
	t.Parallel()

	// AC3: returns false when w is not a terminal.
	t.Run("pipe writer returns false", func(t *testing.T) {
		t.Parallel()
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
	})

	// AC3: returns false for non-*os.File writer.
	t.Run("bytes.Buffer returns false", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer

		ResetColorEnabled()
		defer ResetColorEnabled()

		if ColorEnabled(&buf) {
			t.Error("ColorEnabled(bytes.Buffer) = true, want false")
		}
	})
}

func TestSetColorEnabled(t *testing.T) {
	// AC5: override behavior. Not parallel due to global state.
	defer ResetColorEnabled()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	// Force enabled: pipe should report true.
	SetColorEnabled(true)
	if !ColorEnabled(w) {
		t.Error("SetColorEnabled(true): ColorEnabled(pipe) = false, want true")
	}

	// Force disabled: pipe should report false.
	SetColorEnabled(false)
	if ColorEnabled(w) {
		t.Error("SetColorEnabled(false): ColorEnabled(pipe) = true, want false")
	}

	// Reset: pipe should report false (auto-detect).
	ResetColorEnabled()
	if ColorEnabled(w) {
		t.Error("ResetColorEnabled: ColorEnabled(pipe) = true, want false")
	}
}
