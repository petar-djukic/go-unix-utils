// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/expand against gexpand reference binary.
// Implements prd024-expand R4.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gexpand")
	if err != nil {
		t.Skipf("reference binary gexpand not in PATH: %v", err)
	}

	// Create temp files for multi-file tests.
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "f1.txt")
	file2 := filepath.Join(tmpDir, "f2.txt")
	os.WriteFile(file1, []byte("a\tb\n"), 0o644)   // best-effort, test will fail if this errors
	os.WriteFile(file2, []byte("c\td\n"), 0o644)

	tests := []testutils.DiffTest{
		// R1.1: Default tab expansion at column 1.
		{
			Name:  "default_single_tab",
			Args:  []string{},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: Multiple consecutive tabs.
		{
			Name:  "multiple_consecutive_tabs",
			Args:  []string{},
			Stdin: []byte("\t\tx\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: No tabs — passthrough.
		{
			Name:  "no_tabs_passthrough",
			Args:  []string{},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: Multiline — column resets at newline.
		{
			Name:  "multiline_column_reset",
			Args:  []string{},
			Stdin: []byte("\ta\n\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: Custom interval -t 4.
		{
			Name:  "tab_interval_4",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: Tab stop list -t 1,5,9.
		{
			Name:  "tab_stop_list",
			Args:  []string{"-t", "1,5,9"},
			Stdin: []byte("\ta\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: Tab past last explicit stop replaced by single space.
		{
			Name:  "tab_past_last_stop",
			Args:  []string{"-t", "4"},
			Stdin: []byte("12345678\t9\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// --initial flag: only convert leading tabs.
		{
			Name:  "initial_flag",
			Args:  []string{"-i"},
			Stdin: []byte("\ta\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// --initial with custom tab stop.
		{
			Name:  "initial_with_custom_tab",
			Args:  []string{"-i", "-t", "4"},
			Stdin: []byte("\ta\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Stdin input (no files).
		{
			Name:  "stdin_input",
			Args:  []string{},
			Stdin: []byte("x\ty\tz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Multiple files.
		{
			Name: "multiple_files",
			Args: []string{file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// Empty input.
		{
			Name:  "empty_input",
			Args:  []string{},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// Tab at various positions.
		{
			Name:  "tab_mid_line",
			Args:  []string{},
			Stdin: []byte("abcd\tefgh\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Mixed tabs and spaces.
		{
			Name:  "mixed_tabs_and_spaces",
			Args:  []string{},
			Stdin: []byte("  \t  \tx\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Long option --tabs=.
		{
			Name:  "long_tabs_option",
			Args:  []string{"--tabs=4"},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Tab at end of line.
		{
			Name:  "tab_at_end_of_line",
			Args:  []string{},
			Stdin: []byte("abc\t\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
