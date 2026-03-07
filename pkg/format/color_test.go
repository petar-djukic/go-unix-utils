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
	// R2.3: a bytes.Buffer is not an *os.File, so ColorEnabled returns false.
	var buf bytes.Buffer
	if ColorEnabled(&buf) {
		t.Error("ColorEnabled returned true for bytes.Buffer; want false")
	}
}

func TestColorEnabled_Pipe(t *testing.T) {
	t.Parallel()
	// R2.3: a pipe fd is not a terminal.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if ColorEnabled(w) {
		t.Error("ColorEnabled returned true for pipe write end; want false")
	}
	if ColorEnabled(r) {
		t.Error("ColorEnabled returned true for pipe read end; want false")
	}
}

func TestSetColorEnabled_True(t *testing.T) {
	// Not parallel: mutates package-level override.
	defer ResetColorEnabled()

	// R2.6: SetColorEnabled(true) forces color on regardless of writer type.
	SetColorEnabled(true)
	var buf bytes.Buffer
	if !ColorEnabled(&buf) {
		t.Error("ColorEnabled returned false after SetColorEnabled(true) for bytes.Buffer")
	}

	// Also works for pipe writers.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if !ColorEnabled(w) {
		t.Error("ColorEnabled returned false after SetColorEnabled(true) for pipe")
	}
}

func TestSetColorEnabled_False(t *testing.T) {
	// Not parallel: mutates package-level override.
	defer ResetColorEnabled()

	// R2.6: SetColorEnabled(false) forces color off.
	SetColorEnabled(false)
	var buf bytes.Buffer
	if ColorEnabled(&buf) {
		t.Error("ColorEnabled returned true after SetColorEnabled(false)")
	}
}

func TestResetColorEnabled(t *testing.T) {
	// Not parallel: mutates package-level override.
	defer ResetColorEnabled()

	SetColorEnabled(true)
	ResetColorEnabled()

	// R2.7: after reset, reverts to automatic detection. Pipe is not a terminal.
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

	// R2.1, R2.4: correct ANSI codes for each of the eight file types.
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

	// R2.3: when color is disabled, FileTypeColor returns empty string for all types.
	modes := []os.FileMode{
		os.ModeDir,
		os.ModeSymlink,
		0o755,
		os.ModeDevice,
		os.ModeSocket,
		os.ModeNamedPipe,
		0o644,
	}
	for _, mode := range modes {
		got := FileTypeColor(mode)
		if got != "" {
			t.Errorf("FileTypeColor(%v) with color disabled = %q, want empty", mode, got)
		}
	}
}

func TestFileTypeColor_NoOverride(t *testing.T) {
	// Not parallel: uses ResetColorEnabled.
	defer ResetColorEnabled()
	ResetColorEnabled()

	// Without override, colorActive() returns false, so FileTypeColor returns "".
	got := FileTypeColor(os.ModeDir)
	if got != "" {
		t.Errorf("FileTypeColor(ModeDir) without override = %q, want empty", got)
	}
}

func TestReset_Enabled(t *testing.T) {
	// Not parallel: uses SetColorEnabled.
	defer ResetColorEnabled()
	SetColorEnabled(true)

	// R2.2: Reset returns the ANSI reset sequence when color is enabled.
	got := Reset()
	if got != "\033[0m" {
		t.Errorf("Reset() = %q, want %q", got, "\033[0m")
	}
}

func TestReset_Disabled(t *testing.T) {
	// Not parallel: uses SetColorEnabled.
	defer ResetColorEnabled()
	SetColorEnabled(false)

	// R2.2: Reset returns empty string when color is disabled.
	got := Reset()
	if got != "" {
		t.Errorf("Reset() with color disabled = %q, want empty", got)
	}
}

func TestReset_NoOverride(t *testing.T) {
	// Not parallel: uses ResetColorEnabled.
	defer ResetColorEnabled()
	ResetColorEnabled()

	// Without override, colorActive() returns false.
	got := Reset()
	if got != "" {
		t.Errorf("Reset() without override = %q, want empty", got)
	}
}
