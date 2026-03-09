// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileTypeColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode os.FileMode
		want string
	}{
		{"directory", os.ModeDir, "\033[34m"},
		{"symlink", os.ModeSymlink, "\033[36m"},
		{"executable", 0o755, "\033[32m"},
		{"block device", os.ModeDevice, "\033[33;1m"},
		{"character device", os.ModeDevice | os.ModeCharDevice, "\033[33;1m"},
		{"socket", os.ModeSocket, "\033[35m"},
		{"pipe", os.ModeNamedPipe, "\033[33m"},
		{"regular file", 0o644, ""},
		{"regular file no perms", 0, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FileTypeColor(tc.mode)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFileTypeColor_DistinctValues(t *testing.T) {
	t.Parallel()

	// AC1: FileTypeColor returns distinct non-empty ANSI escape strings for
	// directory, symlink, executable, pipe, and socket. Block and character
	// devices share the same code per R2.4 (both yellow bold).
	modes := []os.FileMode{
		os.ModeDir,
		os.ModeSymlink,
		0o755,
		os.ModeSocket,
		os.ModeNamedPipe,
	}

	seen := make(map[string]bool)
	for _, m := range modes {
		c := FileTypeColor(m)
		require.NotEmpty(t, c, "mode %v should produce a non-empty color", m)
		seen[c] = true
	}
	assert.Equal(t, len(modes), len(seen), "all tested modes should produce distinct colors")
}

func TestReset(t *testing.T) {
	t.Parallel()

	// AC2: Reset returns ESC[0m.
	assert.Equal(t, "\033[0m", Reset())
}

func TestColorEnabled_NonTerminal(t *testing.T) {
	t.Parallel()

	// AC3: ColorEnabled returns false for a non-terminal writer.
	ResetColorEnabled()
	defer ResetColorEnabled()

	var buf bytes.Buffer
	assert.False(t, ColorEnabled(&buf), "bytes.Buffer is not a terminal")
}

func TestColorEnabled_PipeWriter(t *testing.T) {
	t.Parallel()

	// AC3: ColorEnabled returns false for an os.Pipe writer (not a TTY).
	ResetColorEnabled()
	defer ResetColorEnabled()

	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()
	defer w.Close()

	assert.False(t, ColorEnabled(w), "pipe writer is not a terminal")
}

func TestSetColorEnabled_ForceOn(t *testing.T) {
	// AC4: SetColorEnabled(true) forces ColorEnabled to return true for
	// non-terminal writers.
	defer ResetColorEnabled()

	var buf bytes.Buffer
	SetColorEnabled(true)
	assert.True(t, ColorEnabled(&buf), "override should force true")
}

func TestSetColorEnabled_ForceOff(t *testing.T) {
	// AC4: SetColorEnabled(false) forces ColorEnabled to return false.
	defer ResetColorEnabled()

	SetColorEnabled(false)

	// Even an *os.File should return false when override is active.
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()
	defer w.Close()

	assert.False(t, ColorEnabled(w), "override should force false")
}

func TestSetColorEnabled_MultipleCalls(t *testing.T) {
	// AC4: Calling SetColorEnabled multiple times uses the last value set.
	defer ResetColorEnabled()

	var buf bytes.Buffer

	SetColorEnabled(true)
	require.True(t, ColorEnabled(&buf), "first call: true")

	SetColorEnabled(false)
	require.False(t, ColorEnabled(&buf), "second call: false")

	SetColorEnabled(true)
	assert.True(t, ColorEnabled(&buf), "third call: true (last value wins)")
}

func TestResetColorEnabled(t *testing.T) {
	// AC4: ResetColorEnabled restores auto-detection.
	defer ResetColorEnabled()

	SetColorEnabled(true)

	var buf bytes.Buffer
	require.True(t, ColorEnabled(&buf), "override on")

	ResetColorEnabled()
	assert.False(t, ColorEnabled(&buf), "after reset, non-terminal should be false")
}
