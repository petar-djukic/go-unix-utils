// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// color_test.go contains unit tests for FileTypeColor, Reset, SetColorEnabled,
// ResetColorEnabled, and ColorEnabled, verifying ANSI escape sequences for all
// file types, manual override behavior, and automatic TTY detection.
//
// Tests: prd003-format R2.1, R2.2, R2.3, R2.4, R2.5, R2.6, R2.7.
package format

import (
	"bytes"
	"os"
	"testing"
)

func TestFileTypeColor(t *testing.T) {
	// Force color on so FileTypeColor returns ANSI codes unconditionally.
	SetColorEnabled(true)
	t.Cleanup(ResetColorEnabled)

	tests := []struct {
		name     string
		mode     os.FileMode
		expected string
	}{
		{name: "directory", mode: os.ModeDir, expected: "\033[34m"},
		{name: "symlink", mode: os.ModeSymlink, expected: "\033[36m"},
		{name: "executable", mode: 0o755, expected: "\033[32m"},
		{name: "block_device", mode: os.ModeDevice, expected: "\033[33;1m"},
		{name: "char_device", mode: os.ModeDevice | os.ModeCharDevice, expected: "\033[33;1m"},
		{name: "socket", mode: os.ModeSocket, expected: "\033[35m"},
		{name: "pipe", mode: os.ModeNamedPipe, expected: "\033[33m"},
		{name: "regular_file", mode: 0o644, expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FileTypeColor(tt.mode)
			if got != tt.expected {
				t.Errorf("FileTypeColor(%v) = %q, want %q", tt.mode, got, tt.expected)
			}
		})
	}
}

func TestReset(t *testing.T) {
	SetColorEnabled(true)
	t.Cleanup(ResetColorEnabled)

	got := Reset()
	if got != "\033[0m" {
		t.Errorf("Reset() = %q, want %q", got, "\033[0m")
	}
}

func TestSetColorEnabledFalse(t *testing.T) {
	// SetColorEnabled(false) must suppress all ANSI output (R2.6).
	SetColorEnabled(false)
	t.Cleanup(ResetColorEnabled)

	// All file types should return empty string.
	modes := []struct {
		name string
		mode os.FileMode
	}{
		{"directory", os.ModeDir},
		{"symlink", os.ModeSymlink},
		{"executable", 0o755},
		{"block_device", os.ModeDevice},
		{"char_device", os.ModeDevice | os.ModeCharDevice},
		{"socket", os.ModeSocket},
		{"pipe", os.ModeNamedPipe},
		{"regular", 0o644},
	}

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			got := FileTypeColor(m.mode)
			if got != "" {
				t.Errorf("FileTypeColor(%v) with color disabled = %q, want empty string",
					m.mode, got)
			}
		})
	}

	t.Run("reset_suppressed", func(t *testing.T) {
		got := Reset()
		if got != "" {
			t.Errorf("Reset() with color disabled = %q, want empty string", got)
		}
	})
}

func TestSetColorEnabledTrue(t *testing.T) {
	// SetColorEnabled(true) must force ANSI output regardless of writer type (R2.6).
	SetColorEnabled(true)
	t.Cleanup(ResetColorEnabled)

	got := FileTypeColor(os.ModeDir)
	if got != "\033[34m" {
		t.Errorf("FileTypeColor(ModeDir) with color forced on = %q, want %q",
			got, "\033[34m")
	}

	got = Reset()
	if got != "\033[0m" {
		t.Errorf("Reset() with color forced on = %q, want %q", got, "\033[0m")
	}
}

func TestResetColorEnabled(t *testing.T) {
	// After ResetColorEnabled, automatic TTY detection is restored (R2.7).
	// A bytes.Buffer is not a TTY, so ColorEnabled should return false.
	SetColorEnabled(true)
	ResetColorEnabled()

	var buf bytes.Buffer
	got := ColorEnabled(&buf)
	if got {
		t.Errorf("ColorEnabled(&bytes.Buffer{}) after ResetColorEnabled() = true, want false")
	}
}

func TestColorEnabledNonTTY(t *testing.T) {
	// ColorEnabled returns false for non-TTY writers (R2.3).
	ResetColorEnabled()
	t.Cleanup(ResetColorEnabled)

	t.Run("bytes_buffer", func(t *testing.T) {
		var buf bytes.Buffer
		if ColorEnabled(&buf) {
			t.Error("ColorEnabled(&bytes.Buffer{}) = true, want false")
		}
	})

	t.Run("os_pipe", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() failed: %v", err)
		}
		defer r.Close()
		defer w.Close()

		if ColorEnabled(w) {
			t.Error("ColorEnabled(pipe writer) = true, want false")
		}
	})
}
