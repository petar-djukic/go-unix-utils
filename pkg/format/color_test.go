// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"bytes"
	"os"
	"testing"
)

func TestFileTypeColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode os.FileMode
		want string
	}{
		{"directory", os.ModeDir | 0o755, colorBoldBlue},
		{"symlink", os.ModeSymlink | 0o777, colorBoldCyan},
		{"executable", 0o755, colorBoldGreen},
		{"regular_no_exec", 0o644, ""},
		{"setuid", os.ModeSetuid | 0o755, colorWhiteOnRed},
		{"setgid", os.ModeSetgid | 0o755, colorBlackOnYellow},
		{"sticky_dir", os.ModeDir | os.ModeSticky | 0o755, colorWhiteOnBlue},
		{"other_writable_dir", os.ModeDir | 0o757, colorBlackOnGreen},
		{"block_device", os.ModeDevice | 0o660, colorBoldYellowOnBlk},
		{"char_device", os.ModeDevice | os.ModeCharDevice | 0o660, colorBoldYellowOnBlk2},
		{"named_pipe", os.ModeNamedPipe | 0o644, colorYellow},
		{"socket", os.ModeSocket | 0o755, colorBoldMagenta},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FileTypeColor(tc.mode)
			if got != tc.want {
				t.Errorf("FileTypeColor(%v) = %q, want %q", tc.mode, got, tc.want)
			}
		})
	}
}

func TestFileTypeColorPriority(t *testing.T) {
	t.Parallel()

	// D4: setuid takes precedence over executable and directory.
	t.Run("setuid_over_exec", func(t *testing.T) {
		t.Parallel()
		mode := os.ModeSetuid | 0o755
		got := FileTypeColor(mode)
		if got != colorWhiteOnRed {
			t.Errorf("setuid+exec: got %q, want %q", got, colorWhiteOnRed)
		}
	})

	// D4: setgid takes precedence over directory.
	t.Run("setgid_over_dir", func(t *testing.T) {
		t.Parallel()
		mode := os.ModeDir | os.ModeSetgid | 0o755
		got := FileTypeColor(mode)
		if got != colorBlackOnYellow {
			t.Errorf("setgid+dir: got %q, want %q", got, colorBlackOnYellow)
		}
	})

	// D4: sticky takes precedence over other-writable for directories.
	t.Run("sticky_over_other_writable", func(t *testing.T) {
		t.Parallel()
		mode := os.ModeDir | os.ModeSticky | 0o757
		got := FileTypeColor(mode)
		if got != colorWhiteOnBlue {
			t.Errorf("sticky+ow dir: got %q, want %q", got, colorWhiteOnBlue)
		}
	})
}

func TestReset(t *testing.T) {
	t.Parallel()

	// AC2: Reset returns "\033[0m".
	got := Reset()
	want := "\033[0m"
	if got != want {
		t.Errorf("Reset() = %q, want %q", got, want)
	}
}

func TestColorEnabledNonFile(t *testing.T) {
	t.Parallel()

	// AC4: a non-*os.File writer returns false.
	var buf bytes.Buffer
	if ColorEnabled(&buf) {
		t.Error("ColorEnabled(bytes.Buffer) = true, want false")
	}
}

func TestColorEnabledPipe(t *testing.T) {
	t.Parallel()

	// AC4: a pipe (not a TTY) returns false.
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

func TestSetColorEnabled(t *testing.T) {
	// Not parallel: modifies package-level state.

	// AC5: SetColorEnabled(true) forces color on even for a pipe.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	SetColorEnabled(true)
	if !ColorEnabled(w) {
		t.Error("after SetColorEnabled(true): ColorEnabled(pipe) = false, want true")
	}

	// AC6: SetColorEnabled(false) forces color off.
	SetColorEnabled(false)
	if ColorEnabled(w) {
		t.Error("after SetColorEnabled(false): ColorEnabled(pipe) = true, want false")
	}

	// AC7: ResetColorEnabled restores auto-detection.
	ResetColorEnabled()
	if ColorEnabled(w) {
		t.Error("after ResetColorEnabled: ColorEnabled(pipe) = true, want false (pipe is not a TTY)")
	}
}

func TestSetColorEnabledNonFile(t *testing.T) {
	// Not parallel: modifies package-level state.

	var buf bytes.Buffer

	// Override should apply even for non-*os.File writers.
	SetColorEnabled(true)
	if !ColorEnabled(&buf) {
		t.Error("after SetColorEnabled(true): ColorEnabled(bytes.Buffer) = false, want true")
	}

	ResetColorEnabled()
	if ColorEnabled(&buf) {
		t.Error("after ResetColorEnabled: ColorEnabled(bytes.Buffer) = true, want false")
	}
}
