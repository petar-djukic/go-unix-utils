// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for ANSI color output functions.
//
// Implements: prd003-format R2.1, R2.2, R2.3, R2.4, R2.6, R2.7
package format

import (
	"bytes"
	"os"
	"testing"
)

// --- FileTypeColor tests (prd003-format R2.1, R2.4) ---

func TestFileTypeColor(t *testing.T) {
	SetColorEnabled(true)
	t.Cleanup(ResetColorEnabled)

	tests := []struct {
		name   string
		mode   os.FileMode
		expect string
	}{
		{
			name:   "directory",
			mode:   os.ModeDir,
			expect: "\033[34m",
		},
		{
			name:   "symlink",
			mode:   os.ModeSymlink,
			expect: "\033[36m",
		},
		{
			name:   "executable",
			mode:   0o755,
			expect: "\033[32m",
		},
		{
			name:   "block-device",
			mode:   os.ModeDevice,
			expect: "\033[33;1m",
		},
		{
			name:   "char-device",
			mode:   os.ModeDevice | os.ModeCharDevice,
			expect: "\033[33;1m",
		},
		{
			name:   "socket",
			mode:   os.ModeSocket,
			expect: "\033[35m",
		},
		{
			name:   "pipe",
			mode:   os.ModeNamedPipe,
			expect: "\033[33m",
		},
		{
			name:   "regular-file",
			mode:   0o644,
			expect: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FileTypeColor(tc.mode)
			if got != tc.expect {
				t.Errorf("FileTypeColor(%v) = %q, want %q", tc.mode, got, tc.expect)
			}
		})
	}
}

// --- Reset tests (prd003-format R2.2) ---

func TestReset_ColorEnabled(t *testing.T) {
	SetColorEnabled(true)
	t.Cleanup(ResetColorEnabled)

	got := Reset()
	want := "\033[0m"
	if got != want {
		t.Errorf("Reset() = %q, want %q", got, want)
	}
}

func TestReset_ColorDisabled(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(ResetColorEnabled)

	got := Reset()
	if got != "" {
		t.Errorf("Reset() with color disabled = %q, want empty string", got)
	}
}

// --- ColorEnabled tests (prd003-format R2.3) ---

func TestColorEnabled_Pipe(t *testing.T) {
	ResetColorEnabled()
	t.Cleanup(ResetColorEnabled)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if ColorEnabled(w) {
		t.Error("ColorEnabled(pipe) = true, want false")
	}
}

func TestColorEnabled_NonFile(t *testing.T) {
	ResetColorEnabled()
	t.Cleanup(ResetColorEnabled)

	var buf bytes.Buffer
	if ColorEnabled(&buf) {
		t.Error("ColorEnabled(bytes.Buffer) = true, want false")
	}
}

// --- SetColorEnabled / ResetColorEnabled tests (prd003-format R2.6, R2.7) ---

func TestSetColorEnabled_ForceOn(t *testing.T) {
	SetColorEnabled(true)
	t.Cleanup(ResetColorEnabled)

	got := FileTypeColor(os.ModeDir)
	if got == "" {
		t.Error("FileTypeColor(ModeDir) with SetColorEnabled(true) returned empty string, want ANSI code")
	}
}

func TestSetColorEnabled_ForceOff(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(ResetColorEnabled)

	got := FileTypeColor(os.ModeDir)
	if got != "" {
		t.Errorf("FileTypeColor(ModeDir) with SetColorEnabled(false) = %q, want empty string", got)
	}

	got = Reset()
	if got != "" {
		t.Errorf("Reset() with SetColorEnabled(false) = %q, want empty string", got)
	}
}

func TestResetColorEnabled_RevertsToAuto(t *testing.T) {
	SetColorEnabled(true)
	ResetColorEnabled()
	t.Cleanup(ResetColorEnabled)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if ColorEnabled(w) {
		t.Error("ColorEnabled(pipe) after ResetColorEnabled() = true, want false")
	}
}
