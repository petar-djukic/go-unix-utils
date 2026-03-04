// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for prd003-format R2.1-R2.4: FileTypeColor, Reset, ColorEnabled,
// SetColorEnabled, ResetColorEnabled.

package format_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
)

// ANSI escape constants matching GNU ls LS_COLORS defaults (prd003 R2.4).
const (
	ansiBlue       = "\033[34m"   // directory
	ansiCyan       = "\033[36m"   // symlink
	ansiGreen      = "\033[32m"   // executable
	ansiYellowBold = "\033[33;1m" // block device, char device
	ansiMagenta    = "\033[35m"   // socket
	ansiYellow     = "\033[33m"   // pipe
	ansiReset      = "\033[0m"    // regular / reset
)

// TestFileTypeColor_AllEightTypes verifies that FileTypeColor returns the
// correct ANSI escape sequence for each of the eight GNU ls file types.
// R2.1, R2.4: color lookup for directory, symlink, executable, block device,
// char device, socket, pipe, and regular file.
func TestFileTypeColor_AllEightTypes(t *testing.T) {
	format.SetColorEnabled(true)
	defer format.ResetColorEnabled()

	tests := []struct {
		name string
		mode os.FileMode
		want string
	}{
		{"directory", os.ModeDir, ansiBlue},
		{"symlink", os.ModeSymlink, ansiCyan},
		{"executable", 0o755, ansiGreen},
		{"block_device", os.ModeDevice, ansiYellowBold},
		{"char_device", os.ModeDevice | os.ModeCharDevice, ansiYellowBold},
		{"socket", os.ModeSocket, ansiMagenta},
		{"pipe", os.ModeNamedPipe, ansiYellow},
		{"regular", 0o644, ansiReset},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := format.FileTypeColor(tc.mode)
			if got != tc.want {
				t.Errorf("FileTypeColor(%v) = %q, want %q", tc.mode, got, tc.want)
			}
		})
	}
}

// TestFileTypeColor_ColorDisabled verifies that FileTypeColor returns empty
// strings for all file types when color output is disabled.
// R2.1, R2.6: empty strings when SetColorEnabled(false) is active.
func TestFileTypeColor_ColorDisabled(t *testing.T) {
	format.SetColorEnabled(false)
	defer format.ResetColorEnabled()

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
	for _, tc := range modes {
		t.Run(tc.name, func(t *testing.T) {
			got := format.FileTypeColor(tc.mode)
			if got != "" {
				t.Errorf("FileTypeColor(%v) with color disabled = %q, want empty string", tc.mode, got)
			}
		})
	}
}

// TestReset_ColorEnabled verifies that Reset returns the ANSI reset sequence
// when color output is enabled.
// R2.2: reset sequence for terminating color output.
func TestReset_ColorEnabled(t *testing.T) {
	format.SetColorEnabled(true)
	defer format.ResetColorEnabled()

	got := format.Reset()
	if got != ansiReset {
		t.Errorf("Reset() with color enabled = %q, want %q", got, ansiReset)
	}
}

// TestReset_ColorDisabled verifies that Reset returns an empty string when
// color output is disabled.
// R2.2, R2.6: empty string when SetColorEnabled(false) is active.
func TestReset_ColorDisabled(t *testing.T) {
	format.SetColorEnabled(false)
	defer format.ResetColorEnabled()

	got := format.Reset()
	if got != "" {
		t.Errorf("Reset() with color disabled = %q, want empty string", got)
	}
}

// TestColorEnabled_NonFileWriter verifies that ColorEnabled returns false
// when passed an io.Writer that is not backed by *os.File.
// R2.3: type assertion to *os.File fails, returns false.
func TestColorEnabled_NonFileWriter(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	got := format.ColorEnabled(&buf)
	if got {
		t.Error("ColorEnabled(bytes.Buffer) = true, want false")
	}
}

// TestColorEnabled_OsFile verifies that ColorEnabled exercises the *os.File
// TTY detection path. os.Stderr in a test runner is typically not a TTY
// (piped by the test harness), so we expect false. The important check is
// that the code path through the ioctl is reached without panic or error.
// R2.3: automatic TTY detection via TIOCGWINSZ ioctl.
func TestColorEnabled_OsFile(t *testing.T) {
	t.Parallel()

	// os.Stderr is an *os.File, so the type assertion succeeds and the
	// ioctl path is exercised. In CI/test environments this is typically
	// not a TTY, so the result should be false.
	got := format.ColorEnabled(os.Stderr)
	if got {
		t.Error("ColorEnabled(os.Stderr) in test runner = true, want false")
	}
}

// TestColorEnabled_Pipe verifies that ColorEnabled returns false for a pipe
// file descriptor (not a terminal).
// R2.3: pipes are non-terminal descriptors.
func TestColorEnabled_Pipe(t *testing.T) {
	t.Parallel()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if format.ColorEnabled(r) {
		t.Error("ColorEnabled(pipe read end) = true, want false")
	}
	if format.ColorEnabled(w) {
		t.Error("ColorEnabled(pipe write end) = true, want false")
	}
}

// TestSetColorEnabled_Override verifies that SetColorEnabled overrides
// automatic TTY detection for both true and false values, and that
// ResetColorEnabled reverts to automatic detection.
// R2.6, R2.7: process-global override and revert.
func TestSetColorEnabled_Override(t *testing.T) {
	// Force color on — FileTypeColor should return ANSI codes even though
	// stdout is not a TTY in the test runner.
	format.SetColorEnabled(true)
	got := format.FileTypeColor(os.ModeDir)
	if got != ansiBlue {
		t.Errorf("FileTypeColor(dir) with SetColorEnabled(true) = %q, want %q", got, ansiBlue)
	}

	// Force color off — FileTypeColor should return empty strings.
	format.SetColorEnabled(false)
	got = format.FileTypeColor(os.ModeDir)
	if got != "" {
		t.Errorf("FileTypeColor(dir) with SetColorEnabled(false) = %q, want empty string", got)
	}

	// Revert to automatic detection.
	format.ResetColorEnabled()
	// After reset, color depends on TTY status. In a test runner stdout is
	// typically a pipe, so FileTypeColor should return empty.
	got = format.FileTypeColor(os.ModeDir)
	if got != "" {
		t.Errorf("FileTypeColor(dir) after ResetColorEnabled = %q, want empty string (non-TTY)", got)
	}
}

// TestColorConstants_GNUDefaults verifies that the ANSI escape sequences
// returned by FileTypeColor match the GNU ls LS_COLORS defaults exactly.
// R2.4: blue=34, cyan=36, green=32, yellow bold=33;1, magenta=35, yellow=33.
func TestColorConstants_GNUDefaults(t *testing.T) {
	format.SetColorEnabled(true)
	defer format.ResetColorEnabled()

	tests := []struct {
		name     string
		mode     os.FileMode
		wantAnsi string
		desc     string
	}{
		{"directory", os.ModeDir, "\033[34m", "directory must be blue (34)"},
		{"symlink", os.ModeSymlink, "\033[36m", "symlink must be cyan (36)"},
		{"executable", 0o755, "\033[32m", "executable must be green (32)"},
		{"block_device", os.ModeDevice, "\033[33;1m", "block device must be yellow bold (33;1)"},
		{"char_device", os.ModeDevice | os.ModeCharDevice, "\033[33;1m", "char device must be yellow bold (33;1)"},
		{"socket", os.ModeSocket, "\033[35m", "socket must be magenta (35)"},
		{"pipe", os.ModeNamedPipe, "\033[33m", "pipe must be yellow (33)"},
		{"regular", 0o644, "\033[0m", "regular must be reset (0)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := format.FileTypeColor(tc.mode)
			if got != tc.wantAnsi {
				t.Errorf("%s: FileTypeColor(%v) = %q, want %q", tc.desc, tc.mode, got, tc.wantAnsi)
			}
		})
	}
}
