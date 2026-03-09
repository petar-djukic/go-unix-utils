// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"bytes"
	"os"
	"testing"
)

func TestColorEnabledPipe(t *testing.T) {
	// A bytes.Buffer is not an *os.File, so ColorEnabled must return false.
	ResetColorEnabled()
	var buf bytes.Buffer
	if ColorEnabled(&buf) {
		t.Error("ColorEnabled should return false for non-*os.File writer")
	}
}

func TestSetColorEnabledTrue(t *testing.T) {
	defer ResetColorEnabled()

	SetColorEnabled(true)

	// With override true, ColorEnabled returns true even for a non-TTY writer.
	var buf bytes.Buffer
	if !ColorEnabled(&buf) {
		t.Error("ColorEnabled should return true after SetColorEnabled(true)")
	}

	// FileTypeColor should return ANSI code for directories.
	got := FileTypeColor(os.ModeDir)
	if got == "" {
		t.Error("FileTypeColor(ModeDir) should return ANSI code when color forced on")
	}

	// Reset should return ANSI reset sequence.
	if Reset() == "" {
		t.Error("Reset() should return ANSI reset when color forced on")
	}
}

func TestSetColorEnabledFalse(t *testing.T) {
	defer ResetColorEnabled()

	SetColorEnabled(false)

	// With override false, FileTypeColor and Reset return empty strings.
	got := FileTypeColor(os.ModeDir)
	if got != "" {
		t.Errorf("FileTypeColor(ModeDir) = %q, want empty when color forced off", got)
	}
	if Reset() != "" {
		t.Errorf("Reset() = %q, want empty when color forced off", Reset())
	}
}

func TestResetColorEnabled(t *testing.T) {
	// AC1: after SetColorEnabled(true) then ResetColorEnabled(),
	// ColorEnabled returns to auto-detect behavior.
	SetColorEnabled(true)
	ResetColorEnabled()

	// A bytes.Buffer is not a TTY, so auto-detect should return false.
	var buf bytes.Buffer
	if ColorEnabled(&buf) {
		t.Error("after ResetColorEnabled, ColorEnabled should auto-detect (false for non-TTY)")
	}
}

func TestFileTypeColorValues(t *testing.T) {
	defer ResetColorEnabled()
	SetColorEnabled(true)

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
		{"setuid", os.ModeSetuid | 0o755, "\033[37;41m"},
		{"setgid", os.ModeSetgid | 0o755, "\033[30;43m"},
		{"sticky", os.ModeSticky | os.ModeDir, "\033[37;44m"},
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
