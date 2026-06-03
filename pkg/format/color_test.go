// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"bytes"
	"os"
	"testing"
)

func TestFileTypeColor(t *testing.T) {
	tests := []struct {
		name string
		mode os.FileMode
		want string
	}{
		{"directory", os.ModeDir, "\033[01;34m"},
		{"symlink", os.ModeSymlink, "\033[01;36m"},
		{"named_pipe", os.ModeNamedPipe, "\033[40;33m"},
		{"socket", os.ModeSocket, "\033[01;35m"},
		{"char_device", os.ModeDevice | os.ModeCharDevice, "\033[01;33m"},
		{"block_device", os.ModeDevice, "\033[01;33m"},
		{"executable", os.FileMode(0o755), "\033[01;32m"},
		{"regular", os.FileMode(0o644), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FileTypeColor(tt.mode); got != tt.want {
				t.Errorf("FileTypeColor(%v) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestReset(t *testing.T) {
	want := "\033[0m"
	if got := Reset(); got != want {
		t.Errorf("Reset() = %q, want %q", got, want)
	}
}

func TestColorEnabledBuffer(t *testing.T) {
	var buf bytes.Buffer
	if ColorEnabled(&buf) {
		t.Error("ColorEnabled(bytes.Buffer) = true, want false")
	}
}

func TestColorEnabledPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if ColorEnabled(w) {
		t.Error("ColorEnabled(os.Pipe writer) = true, want false")
	}
}

func TestSetColorEnabledTrue(t *testing.T) {
	defer ResetColorEnabled()

	SetColorEnabled(true)
	if !ColorEnabled(&bytes.Buffer{}) {
		t.Error("after SetColorEnabled(true), ColorEnabled(buffer) = false")
	}
	if got := FileTypeColor(os.ModeDir); got == "" {
		t.Error("after SetColorEnabled(true), FileTypeColor(Dir) = empty")
	}
	if got := Reset(); got == "" {
		t.Error("after SetColorEnabled(true), Reset() = empty")
	}
}

func TestSetColorEnabledFalse(t *testing.T) {
	defer ResetColorEnabled()

	SetColorEnabled(false)
	if ColorEnabled(os.Stdout) {
		t.Error("after SetColorEnabled(false), ColorEnabled(Stdout) = true")
	}
	if got := FileTypeColor(os.ModeDir); got != "" {
		t.Errorf("after SetColorEnabled(false), FileTypeColor(Dir) = %q, want empty", got)
	}
	if got := Reset(); got != "" {
		t.Errorf("after SetColorEnabled(false), Reset() = %q, want empty", got)
	}
}

func TestResetColorEnabled(t *testing.T) {
	defer ResetColorEnabled()

	SetColorEnabled(false)
	ResetColorEnabled()

	var buf bytes.Buffer
	if ColorEnabled(&buf) {
		t.Error("after ResetColorEnabled, ColorEnabled(buffer) = true")
	}
	if got := FileTypeColor(os.ModeDir); got == "" {
		t.Error("after ResetColorEnabled, FileTypeColor(Dir) = empty")
	}
}
