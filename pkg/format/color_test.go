// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for color.go: FileTypeColor, Reset, ColorEnabled, SetColorEnabled, ResetColorEnabled.
// Implements: prd003-format (R2)
package format

import (
	"bytes"
	"os"
	"testing"
)

func TestFileTypeColor(t *testing.T) {
	// Force color on so FileTypeColor returns ANSI sequences regardless of TTY.
	// (prd003-format R2.6)
	SetColorEnabled(true)
	t.Cleanup(ResetColorEnabled)

	tests := []struct {
		name string
		mode os.FileMode
		want string
	}{
		{
			name: "directory",
			mode: os.ModeDir | 0755,
			want: "\033[34m",
		},
		{
			name: "symlink",
			mode: os.ModeSymlink | 0777,
			want: "\033[36m",
		},
		{
			name: "executable regular file",
			mode: 0755,
			want: "\033[32m",
		},
		{
			name: "regular file non-executable",
			mode: 0644,
			want: "\033[0m",
		},
		{
			name: "block device",
			mode: os.ModeDevice | 0660,
			want: "\033[33;1m",
		},
		{
			name: "character device",
			mode: os.ModeDevice | os.ModeCharDevice | 0666,
			want: "\033[33;1m",
		},
		{
			name: "socket",
			mode: os.ModeSocket | 0755,
			want: "\033[35m",
		},
		{
			name: "named pipe",
			mode: os.ModeNamedPipe | 0644,
			want: "\033[33m",
		},
		{
			name: "setuid overrides file type",
			mode: os.ModeSetuid | 0755,
			want: "\033[37;41m",
		},
		{
			name: "setgid overrides file type",
			mode: os.ModeSetgid | 0755,
			want: "\033[30;43m",
		},
		{
			name: "sticky directory",
			mode: os.ModeDir | os.ModeSticky | 0755,
			want: "\033[37;44m",
		},
		{
			name: "other-writable directory",
			mode: os.ModeDir | 0757, // o+w bit set
			want: "\033[34;42m",
		},
		{
			name: "sticky and other-writable directory",
			mode: os.ModeDir | os.ModeSticky | 0757,
			want: "\033[30;42m",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FileTypeColor(tc.mode)
			if got != tc.want {
				t.Fatalf("FileTypeColor(%v) = %q, want %q", tc.mode, got, tc.want)
			}
		})
	}
}

func TestFileTypeColorDisabled(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(ResetColorEnabled)

	got := FileTypeColor(os.ModeDir | 0755)
	if got != "" {
		t.Fatalf("FileTypeColor with color disabled = %q, want empty string", got)
	}
}

func TestReset(t *testing.T) {
	t.Run("returns reset sequence when color enabled", func(t *testing.T) {
		SetColorEnabled(true)
		t.Cleanup(ResetColorEnabled)

		got := Reset()
		want := "\033[0m"
		if got != want {
			t.Fatalf("Reset() = %q, want %q", got, want)
		}
	})

	t.Run("returns empty string when color disabled", func(t *testing.T) {
		SetColorEnabled(false)
		t.Cleanup(ResetColorEnabled)

		got := Reset()
		if got != "" {
			t.Fatalf("Reset() with color disabled = %q, want empty string", got)
		}
	})
}

func TestColorEnabled(t *testing.T) {
	t.Run("returns false for pipe", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe: %v", err)
		}
		defer r.Close()
		defer w.Close()

		got := ColorEnabled(w)
		if got {
			t.Fatal("ColorEnabled(pipe) = true, want false")
		}
	})

	t.Run("returns false for non-file writer", func(t *testing.T) {
		var buf bytes.Buffer
		got := ColorEnabled(&buf)
		if got {
			t.Fatal("ColorEnabled(bytes.Buffer) = true, want false")
		}
	})
}

func TestSetColorEnabled(t *testing.T) {
	t.Run("force on overrides TTY detection", func(t *testing.T) {
		SetColorEnabled(true)
		t.Cleanup(ResetColorEnabled)

		// FileTypeColor should return ANSI even though we're not in a TTY.
		got := FileTypeColor(os.ModeDir | 0755)
		if got == "" {
			t.Fatal("FileTypeColor with SetColorEnabled(true) returned empty string")
		}
	})

	t.Run("force off suppresses color", func(t *testing.T) {
		SetColorEnabled(false)
		t.Cleanup(ResetColorEnabled)

		got := FileTypeColor(os.ModeDir | 0755)
		if got != "" {
			t.Fatalf("FileTypeColor with SetColorEnabled(false) = %q, want empty string", got)
		}

		got = Reset()
		if got != "" {
			t.Fatalf("Reset with SetColorEnabled(false) = %q, want empty string", got)
		}
	})
}

func TestResetColorEnabled(t *testing.T) {
	// Force color on, then reset to auto.
	SetColorEnabled(true)
	ResetColorEnabled()

	// After reset, behavior depends on TTY detection. In test environments
	// stdout is typically not a TTY, so colorActive() should return false.
	// We verify by checking with a pipe writer through ColorEnabled.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	got := ColorEnabled(w)
	if got {
		t.Fatal("ColorEnabled(pipe) after ResetColorEnabled = true, want false")
	}
}
