// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for FileTypeColor, Reset, ColorEnabled, SetColorEnabled, ResetColorEnabled
// (prd003-format R2.1, R2.2, R2.3, R2.4, R2.6, R2.7).
package format

import (
	"bytes"
	"os"
	"testing"
)

func TestFileTypeColor(t *testing.T) {
	SetColorEnabled(true)
	t.Cleanup(ResetColorEnabled)

	tests := []struct {
		name string
		mode os.FileMode
		want string
	}{
		{
			name: "directory",
			mode: os.ModeDir | 0o755,
			want: "\033[34m",
		},
		{
			name: "symlink",
			mode: os.ModeSymlink,
			want: "\033[36m",
		},
		{
			name: "executable regular file",
			mode: 0o755,
			want: "\033[32m",
		},
		{
			name: "block device",
			mode: os.ModeDevice | 0o660,
			want: "\033[33;1m",
		},
		{
			name: "character device",
			mode: os.ModeDevice | os.ModeCharDevice | 0o660,
			want: "\033[33;1m",
		},
		{
			name: "socket",
			mode: os.ModeSocket | 0o755,
			want: "\033[35m",
		},
		{
			name: "named pipe",
			mode: os.ModeNamedPipe | 0o644,
			want: "\033[33m",
		},
		{
			name: "non-executable regular file",
			mode: 0o644,
			want: "\033[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FileTypeColor(tt.mode)
			if got != tt.want {
				t.Errorf("FileTypeColor(%v) = %q; want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestFileTypeColorDisabled(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(ResetColorEnabled)

	tests := []struct {
		name string
		mode os.FileMode
	}{
		{name: "directory", mode: os.ModeDir | 0o755},
		{name: "symlink", mode: os.ModeSymlink},
		{name: "executable regular file", mode: 0o755},
		{name: "regular file", mode: 0o644},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FileTypeColor(tt.mode)
			if got != "" {
				t.Errorf("FileTypeColor(%v) = %q; want empty string when color disabled", tt.mode, got)
			}
		})
	}
}

func TestReset(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		want    string
	}{
		{
			name:    "returns ANSI reset when color enabled",
			enabled: true,
			want:    "\033[0m",
		},
		{
			name:    "returns empty string when color disabled",
			enabled: false,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetColorEnabled(tt.enabled)
			t.Cleanup(ResetColorEnabled)

			got := Reset()
			if got != tt.want {
				t.Errorf("Reset() = %q; want %q", got, tt.want)
			}
		})
	}
}

func TestColorEnabled(t *testing.T) {
	// Clear any override so ColorEnabled uses real TTY detection.
	ResetColorEnabled()
	t.Cleanup(ResetColorEnabled)

	t.Run("pipe is not a TTY", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() failed: %v", err)
		}
		defer r.Close()
		defer w.Close()

		if ColorEnabled(r) {
			t.Error("ColorEnabled(pipe read end) = true; want false")
		}
		if ColorEnabled(w) {
			t.Error("ColorEnabled(pipe write end) = true; want false")
		}
	})

	t.Run("regular file is not a TTY", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "color-test-*")
		if err != nil {
			t.Fatalf("os.CreateTemp() failed: %v", err)
		}
		defer f.Close()

		if ColorEnabled(f) {
			t.Error("ColorEnabled(regular file) = true; want false")
		}
	})

	t.Run("non-os.File writer returns false", func(t *testing.T) {
		var buf bytes.Buffer
		if ColorEnabled(&buf) {
			t.Error("ColorEnabled(bytes.Buffer) = true; want false")
		}
	})
}

func TestSetColorEnabledOverride(t *testing.T) {
	t.Run("force on overrides non-TTY", func(t *testing.T) {
		SetColorEnabled(true)
		t.Cleanup(ResetColorEnabled)

		// FileTypeColor should return a sequence even though we're not in a TTY.
		got := FileTypeColor(os.ModeDir | 0o755)
		if got == "" {
			t.Error("FileTypeColor(directory) = empty; want ANSI sequence with SetColorEnabled(true)")
		}
	})

	t.Run("force off suppresses output", func(t *testing.T) {
		SetColorEnabled(false)
		t.Cleanup(ResetColorEnabled)

		got := FileTypeColor(os.ModeDir | 0o755)
		if got != "" {
			t.Errorf("FileTypeColor(directory) = %q; want empty string with SetColorEnabled(false)", got)
		}

		got = Reset()
		if got != "" {
			t.Errorf("Reset() = %q; want empty string with SetColorEnabled(false)", got)
		}
	})

	t.Run("ResetColorEnabled reverts to automatic detection", func(t *testing.T) {
		SetColorEnabled(true)
		ResetColorEnabled()

		// With automatic detection and a pipe (test runner stdout is typically not a
		// TTY), FileTypeColor should return empty since the test process stdout is not
		// a terminal.
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() failed: %v", err)
		}
		defer r.Close()
		defer w.Close()

		// Verify ColorEnabled returns false for a pipe after reset.
		if ColorEnabled(w) {
			t.Error("ColorEnabled(pipe) = true after ResetColorEnabled; want false")
		}
	})
}
