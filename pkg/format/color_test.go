// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
)

// TestFileTypeColor verifies ANSI codes for all eight file types per R2.4.
// Calls SetColorEnabled(true) to bypass TTY detection in test environments.
// (prd003-format R2.1, R2.4; use case F4, S3)
func TestFileTypeColor(t *testing.T) {
	format.SetColorEnabled(true)
	defer format.ResetColorEnabled()

	tests := []struct {
		name     string
		mode     os.FileMode
		expected string
	}{
		{"directory", os.ModeDir, "\033[34m"},
		{"symlink", os.ModeSymlink, "\033[36m"},
		{"block device", os.ModeDevice, "\033[33;1m"},
		{"char device", os.ModeDevice | os.ModeCharDevice, "\033[33;1m"},
		{"socket", os.ModeSocket, "\033[35m"},
		{"pipe", os.ModeNamedPipe, "\033[33m"},
		{"executable", 0o755, "\033[32m"},
		// regular=0 (reset/default) per R2.4
		{"regular", 0o644, "\033[0m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := format.FileTypeColor(tt.mode)
			if got != tt.expected {
				t.Errorf("FileTypeColor(%v) = %q, want %q", tt.mode, got, tt.expected)
			}
		})
	}
}

// TestReset verifies Reset returns the ANSI reset sequence when color is enabled.
// (prd003-format R2.2)
func TestReset(t *testing.T) {
	format.SetColorEnabled(true)
	defer format.ResetColorEnabled()

	got := format.Reset()
	if got != "\033[0m" {
		t.Errorf("Reset() = %q, want %q", got, "\033[0m")
	}
}

// TestSetColorEnabled_ForcedOff verifies that SetColorEnabled(false) suppresses
// all ANSI output regardless of TTY state. (prd003-format R2.6; AC5, use case F5)
func TestSetColorEnabled_ForcedOff(t *testing.T) {
	format.SetColorEnabled(false)
	defer format.ResetColorEnabled()

	if got := format.FileTypeColor(os.ModeDir); got != "" {
		t.Errorf("FileTypeColor with color=false: got %q, want empty", got)
	}
	if got := format.Reset(); got != "" {
		t.Errorf("Reset with color=false: got %q, want empty", got)
	}
}

// TestSetColorEnabled_ForcedOn verifies that SetColorEnabled(true) enables
// ANSI output even in non-TTY environments. (prd003-format R2.6; AC5)
func TestSetColorEnabled_ForcedOn(t *testing.T) {
	format.SetColorEnabled(true)
	defer format.ResetColorEnabled()

	got := format.FileTypeColor(os.ModeDir)
	if got == "" {
		t.Error("FileTypeColor with color=true: got empty string, want ANSI sequence")
	}
	if got := format.Reset(); got == "" {
		t.Error("Reset with color=true: got empty string, want ANSI sequence")
	}
}

// TestResetColorEnabled verifies that ResetColorEnabled reverts to automatic
// TTY detection and that pipes are then detected as non-terminals.
// (prd003-format R2.7; use case F5, S4)
func TestResetColorEnabled(t *testing.T) {
	format.SetColorEnabled(true)
	format.ResetColorEnabled()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() {
		_ = r.Close() // best-effort test cleanup
		_ = w.Close() // best-effort test cleanup
	})

	if format.ColorEnabled(w) {
		t.Error("ColorEnabled(pipe) = true after ResetColorEnabled, want false")
	}
}

// TestColorEnabled_Pipe verifies ColorEnabled returns false for an os.Pipe
// writer, which is not a terminal. (prd003-format R2.3; AC3, use case F6, S5)
func TestColorEnabled_Pipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() {
		_ = r.Close() // best-effort test cleanup
		_ = w.Close() // best-effort test cleanup
	})

	if format.ColorEnabled(w) {
		t.Error("ColorEnabled(os.Pipe writer) = true, want false")
	}
}

// TestColorEnabled_Buffer verifies ColorEnabled returns false for any
// non-*os.File writer. (prd003-format R2.3; use case F6, S5)
func TestColorEnabled_Buffer(t *testing.T) {
	var buf bytes.Buffer
	if format.ColorEnabled(&buf) {
		t.Error("ColorEnabled(bytes.Buffer) = true, want false")
	}
}
