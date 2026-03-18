// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"bytes"
	"os"
	"testing"
)

// Tests in this file mutate the package-level colorOverride variable via
// SetColorEnabled/ResetColorEnabled, so they must NOT call t.Parallel().

func TestFileTypeSequence(t *testing.T) {
	// fileTypeSequence is a pure function — subtests can be parallel.
	tests := []struct {
		name string
		mode os.FileMode
		want string
	}{
		{"directory", os.ModeDir, "\033[34m"},
		{"symlink", os.ModeSymlink, "\033[36m"},
		{"executable", 0o755, "\033[32m"},
		{"block_device", os.ModeDevice, "\033[33;1m"},
		{"char_device", os.ModeDevice | os.ModeCharDevice, "\033[33;1m"},
		{"socket", os.ModeSocket, "\033[35m"},
		{"named_pipe", os.ModeNamedPipe, "\033[33m"},
		{"regular_file", 0o644, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := fileTypeSequence(tc.mode)
			if got != tc.want {
				t.Errorf("fileTypeSequence(%v) = %q, want %q", tc.mode, got, tc.want)
			}
		})
	}
}

func TestFileTypeColorEnabled(t *testing.T) {
	SetColorEnabled(true)
	t.Cleanup(ResetColorEnabled)

	got := FileTypeColor(os.ModeDir)
	if got != "\033[34m" {
		t.Errorf("FileTypeColor(ModeDir) = %q, want %q", got, "\033[34m")
	}
}

func TestFileTypeColorDisabled(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(ResetColorEnabled)

	got := FileTypeColor(os.ModeDir)
	if got != "" {
		t.Errorf("FileTypeColor with color disabled = %q, want empty", got)
	}
}

func TestResetEnabled(t *testing.T) {
	SetColorEnabled(true)
	t.Cleanup(ResetColorEnabled)

	got := Reset()
	if got != "\033[0m" {
		t.Errorf("Reset() = %q, want %q", got, "\033[0m")
	}
}

func TestResetDisabled(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(ResetColorEnabled)

	got := Reset()
	if got != "" {
		t.Errorf("Reset() with color disabled = %q, want empty", got)
	}
}

func TestColorEnabledNonTerminal(t *testing.T) {
	ResetColorEnabled()
	t.Cleanup(ResetColorEnabled)

	var buf bytes.Buffer
	if ColorEnabled(&buf) {
		t.Error("ColorEnabled(bytes.Buffer) = true, want false")
	}
}

func TestColorEnabledOverrideTrue(t *testing.T) {
	SetColorEnabled(true)
	t.Cleanup(ResetColorEnabled)

	var buf bytes.Buffer
	if !ColorEnabled(&buf) {
		t.Error("ColorEnabled with override true = false, want true")
	}
}

func TestColorEnabledOverrideFalse(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(ResetColorEnabled)

	if ColorEnabled(os.Stdout) {
		t.Error("ColorEnabled with override false = true, want false")
	}
}

func TestResetColorEnabled(t *testing.T) {
	SetColorEnabled(true)
	ResetColorEnabled()

	var buf bytes.Buffer
	if ColorEnabled(&buf) {
		t.Error("ColorEnabled after ResetColorEnabled = true, want false")
	}
}
