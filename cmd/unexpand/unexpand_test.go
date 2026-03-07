// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/unexpand against gunexpand reference binary.
// Implements prd025-unexpand R4.
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
	refBin, err := exec.LookPath("gunexpand")
	if err != nil {
		t.Skipf("reference binary gunexpand not in PATH: %v", err)
	}

	// Create temp files for multi-file tests.
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "f1.txt")
	file2 := filepath.Join(tmpDir, "f2.txt")
	os.WriteFile(file1, []byte("        a\n"), 0o644) // best-effort, test will fail if this errors
	os.WriteFile(file2, []byte("        b\n"), 0o644)

	tests := []testutils.DiffTest{
		// R1.1: Leading 8 spaces become one tab.
		{
			Name:  "leading_8_spaces_to_tab",
			Args:  []string{},
			Stdin: []byte("        text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: Non-leading spaces unchanged in default mode.
		{
			Name:  "non_leading_spaces_default",
			Args:  []string{},
			Stdin: []byte("a        b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: Partial space run (not reaching tab stop) kept as spaces.
		{
			Name:  "partial_spaces_kept",
			Args:  []string{},
			Stdin: []byte("   text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: Existing tabs in leading whitespace.
		{
			Name:  "existing_tabs_leading",
			Args:  []string{},
			Stdin: []byte("\t        text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -a converts all runs of spaces.
		{
			Name:  "all_mode_converts_non_leading",
			Args:  []string{"-a"},
			Stdin: []byte("a        b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: Single space kept with -a.
		{
			Name:  "all_mode_single_space_kept",
			Args:  []string{"-a"},
			Stdin: []byte("a b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -a with leading spaces.
		{
			Name:  "all_mode_leading_spaces",
			Args:  []string{"-a"},
			Stdin: []byte("        text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: Custom tab stop -t 4.
		{
			Name:  "custom_tab_4",
			Args:  []string{"-t", "4"},
			Stdin: []byte("    text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: Custom tab stop with non-leading spaces (-t implies -a).
		{
			Name:  "custom_tab_implies_all",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a   b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: Tab stop list.
		{
			Name:  "tab_stop_list",
			Args:  []string{"-t", "4,8,12"},
			Stdin: []byte("    text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// --first-only flag: only convert leading whitespace.
		{
			Name:  "first_only_flag",
			Args:  []string{"--first-only"},
			Stdin: []byte("        a        b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// --all long option.
		{
			Name:  "long_all_option",
			Args:  []string{"--all"},
			Stdin: []byte("a        b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// --tabs= long option.
		{
			Name:  "long_tabs_option",
			Args:  []string{"--tabs=4"},
			Stdin: []byte("    a\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Empty input.
		{
			Name:  "empty_input",
			Args:  []string{},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// No spaces — passthrough.
		{
			Name:  "no_spaces_passthrough",
			Args:  []string{},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Multiple lines.
		{
			Name:  "multiline",
			Args:  []string{},
			Stdin: []byte("        a\n        b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Mixed tabs and spaces in leading whitespace.
		{
			Name:  "mixed_tabs_spaces_leading",
			Args:  []string{},
			Stdin: []byte("  \t     text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Multiple files.
		{
			Name: "multiple_files",
			Args: []string{file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// Stdin with -a and multiple space runs.
		{
			Name:  "all_mode_multiple_runs",
			Args:  []string{"-a"},
			Stdin: []byte("        a        b        c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Spaces at end of line.
		{
			Name:  "trailing_spaces_default",
			Args:  []string{},
			Stdin: []byte("text        \n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Trailing spaces with -a.
		{
			Name:  "trailing_spaces_all",
			Args:  []string{"-a"},
			Stdin: []byte("text        \n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
