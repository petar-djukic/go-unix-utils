// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"bytes"
	"os"
	"testing"
)

func TestColorEnabled_NonTerminal(t *testing.T) {
	t.Parallel()
	// AC5: a bytes.Buffer is not a terminal.
	var buf bytes.Buffer
	if ColorEnabled(&buf) {
		t.Error("ColorEnabled returned true for bytes.Buffer; want false")
	}
}

func TestColorEnabled_Pipe(t *testing.T) {
	t.Parallel()
	// AC5: a pipe fd is not a terminal.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if ColorEnabled(w) {
		t.Error("ColorEnabled returned true for pipe; want false")
	}
}

func TestSetColorEnabled(t *testing.T) {
	// Not parallel: mutates package-level override.
	defer ResetColorEnabled()

	// AC5: SetColorEnabled(true) forces color on.
	SetColorEnabled(true)
	var buf bytes.Buffer
	if !ColorEnabled(&buf) {
		t.Error("ColorEnabled returned false after SetColorEnabled(true)")
	}

	// AC5: SetColorEnabled(false) forces color off.
	SetColorEnabled(false)
	if ColorEnabled(&buf) {
		t.Error("ColorEnabled returned true after SetColorEnabled(false)")
	}
}

func TestResetColorEnabled(t *testing.T) {
	// Not parallel: mutates package-level override.
	defer ResetColorEnabled()

	SetColorEnabled(true)
	ResetColorEnabled()

	// After reset, pipe should not be a terminal.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if ColorEnabled(w) {
		t.Error("ColorEnabled returned true after ResetColorEnabled for pipe")
	}
}

func TestFileTypeColor(t *testing.T) {
	// Not parallel: uses SetColorEnabled.
	defer ResetColorEnabled()
	SetColorEnabled(true)

	// AC3: distinct ANSI codes for each file type.
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
		{"regular", 0o644, "\033[0m"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FileTypeColor(tc.mode)
			if got != tc.want {
				t.Errorf("FileTypeColor(%v) = %q, want %q", tc.mode, got, tc.want)
			}
		})
	}
}

func TestFileTypeColor_Disabled(t *testing.T) {
	// Not parallel: uses SetColorEnabled.
	defer ResetColorEnabled()
	SetColorEnabled(false)

	got := FileTypeColor(os.ModeDir)
	if got != "" {
		t.Errorf("FileTypeColor with color disabled = %q, want empty", got)
	}
}

func TestReset(t *testing.T) {
	// Not parallel: uses SetColorEnabled.
	defer ResetColorEnabled()

	// AC4: Reset returns "\033[0m" when color is enabled.
	SetColorEnabled(true)
	got := Reset()
	if got != "\033[0m" {
		t.Errorf("Reset() = %q, want %q", got, "\033[0m")
	}

	// Returns empty when disabled.
	SetColorEnabled(false)
	got = Reset()
	if got != "" {
		t.Errorf("Reset() with color disabled = %q, want empty", got)
	}
}
