// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"bytes"
	"os"
	"testing"
)

// withColorEnabled sets the color override and registers a cleanup that
// restores automatic detection after the test completes.
func withColorEnabled(t *testing.T, enabled bool) {
	t.Helper()
	SetColorEnabled(enabled)
	t.Cleanup(ResetColorEnabled)
}

// TestFileTypeColor verifies that FileTypeColor returns the correct ANSI
// escape sequence for all eight file types when color is forced on (use
// case F4, prd003-format R2.4, AC2, AC3).
//
// These tests share the colorState global and must NOT be parallelized.
func TestFileTypeColor(t *testing.T) {
	withColorEnabled(t, true)

	tests := []struct {
		name string
		mode os.FileMode
		want string
	}{
		{"directory", os.ModeDir, ansiDir},
		{"symlink", os.ModeSymlink, ansiLink},
		{"char device", os.ModeDevice | os.ModeCharDevice, ansiCharDev},
		{"block device", os.ModeDevice, ansiBlockDev},
		{"socket", os.ModeSocket, ansiSocket},
		{"named pipe", os.ModeNamedPipe, ansiPipe},
		{"executable", 0o755, ansiExec},
		{"regular file", 0o644, ansiReset},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FileTypeColor(tc.mode)
			if got == "" {
				t.Errorf("FileTypeColor(%v) = %q, want non-empty ANSI sequence", tc.mode, got)
			}
			if got != tc.want {
				t.Errorf("FileTypeColor(%v) = %q, want %q", tc.mode, got, tc.want)
			}
		})
	}
}

// TestColorDisabled verifies that SetColorEnabled(false) suppresses all ANSI
// output from FileTypeColor and Reset (use case F5, prd003-format R2.6, AC5).
//
// This test mutates colorState and must NOT be parallelized.
func TestColorDisabled(t *testing.T) {
	withColorEnabled(t, false)

	modes := []os.FileMode{
		os.ModeDir,
		os.ModeSymlink,
		os.ModeDevice | os.ModeCharDevice,
		os.ModeDevice,
		os.ModeSocket,
		os.ModeNamedPipe,
		0o755,
		0o644,
	}
	for _, mode := range modes {
		if got := FileTypeColor(mode); got != "" {
			t.Errorf("FileTypeColor(%v) with color disabled = %q, want \"\"", mode, got)
		}
	}
	if got := Reset(); got != "" {
		t.Errorf("Reset() with color disabled = %q, want \"\"", got)
	}
}

// TestResetColorEnabled verifies that ResetColorEnabled clears any override
// set by SetColorEnabled (use case F5, prd003-format R2.7).
//
// This test mutates colorState and must NOT be parallelized.
func TestResetColorEnabled(t *testing.T) {
	// Force on, then reset.
	SetColorEnabled(true)
	if colorState == nil {
		t.Fatal("expected colorState to be non-nil after SetColorEnabled(true)")
	}
	ResetColorEnabled()
	if colorState != nil {
		t.Errorf("expected colorState to be nil after ResetColorEnabled, got %v", colorState)
	}
}

// TestColorEnabledPipe verifies that ColorEnabled returns false for an
// os.Pipe writer (not a TTY) and for a bytes.Buffer (not *os.File)
// (use case F6, prd003-format R2.3).
//
// This test reads colorState (which must be nil/auto) and must NOT be
// parallelized, as other tests may concurrently set colorState.
func TestColorEnabledPipe(t *testing.T) {
	// Ensure no override is active.
	ResetColorEnabled()

	// os.Pipe write end is *os.File but not a TTY.
	_, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() }) // best-effort cleanup, error ignored
	if ColorEnabled(w) {
		t.Error("ColorEnabled(pipe writer) = true, want false")
	}

	// bytes.Buffer is not *os.File — type assertion fails.
	var buf bytes.Buffer
	if ColorEnabled(&buf) {
		t.Error("ColorEnabled(*bytes.Buffer) = true, want false")
	}
}

// TestReset verifies that Reset returns a non-empty string when color is
// enabled and an empty string when disabled (prd003-format R2.2).
//
// This test mutates colorState and must NOT be parallelized.
func TestReset(t *testing.T) {
	withColorEnabled(t, true)
	if got := Reset(); got != ansiReset {
		t.Errorf("Reset() with color enabled = %q, want %q", got, ansiReset)
	}

	SetColorEnabled(false)
	if got := Reset(); got != "" {
		t.Errorf("Reset() with color disabled = %q, want \"\"", got)
	}
}
