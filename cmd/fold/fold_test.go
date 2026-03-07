// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/fold against gfold reference binary.
// Implements prd023-fold R1-R4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfold")
	if err != nil {
		t.Skipf("reference binary gfold not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.2: Short line passes through unchanged.
		{
			Name:  "short_line_unchanged",
			Args:  []string{},
			Stdin: []byte("short line\n"),
		},
		// R1.1, R1.3: Wrap at default width 80.
		{
			Name:  "default_width_80",
			Args:  []string{},
			Stdin: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaXXXX\n"),
		},
		// R1.1, R1.3: Wrap at specified width.
		{
			Name:  "wrap_at_width_4",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefghij"),
		},
		// R1.4: Final segment retains trailing newline.
		{
			Name:  "wrap_with_trailing_newline",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefgh\n"),
		},
		// R1.4: No trailing newline preserved.
		{
			Name:  "wrap_no_trailing_newline",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefgh"),
		},
		// R2.1: -w sets width.
		{
			Name:  "width_10",
			Args:  []string{"-w", "10"},
			Stdin: []byte("1234567890abcdef\n"),
		},
		// R2.2: Tab expands to next tab stop for column counting.
		{
			Name:  "tab_column_expansion",
			Args:  []string{"-w", "9"},
			Stdin: []byte("a\tbcde\n"),
		},
		// R2.2: Tab at various positions.
		{
			Name:  "tab_at_column_0",
			Args:  []string{"-w", "10"},
			Stdin: []byte("\tabcdefgh\n"),
		},
		// R2.3: -b counts bytes, tabs are 1 byte.
		{
			Name:  "byte_mode",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("abcde"),
		},
		// R2.3: -b with tab (tab counts as 1 byte).
		{
			Name:  "byte_mode_tab",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("a\tbcdefg\n"),
		},
		// R3.1: -s breaks at last space within width.
		{
			Name:  "space_break",
			Args:  []string{"-w", "11", "-s"},
			Stdin: []byte("hello world foo\n"),
		},
		// R3.2: -s falls back to hard wrap when no space found.
		{
			Name:  "space_break_no_space",
			Args:  []string{"-w", "4", "-s"},
			Stdin: []byte("abcdefgh\n"),
		},
		// R3.1: -s with multiple words.
		{
			Name:  "space_break_multiple_words",
			Args:  []string{"-w", "10", "-s"},
			Stdin: []byte("one two three four five\n"),
		},
		// R3.4: -s with -b.
		{
			Name:  "space_break_byte_mode",
			Args:  []string{"-s", "-b", "-w", "10"},
			Stdin: []byte("hello world foo bar\n"),
		},
		// Combined flags: -bs -w.
		{
			Name:  "combined_bs_flags",
			Args:  []string{"-bsw", "5"},
			Stdin: []byte("ab cd efgh\n"),
		},
		// Multiple lines.
		{
			Name:  "multiple_lines",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcdefgh\n1234567890\n"),
		},
		// Empty input.
		{
			Name:  "empty_input",
			Args:  []string{},
			Stdin: []byte(""),
		},
		// Single newline.
		{
			Name:  "single_newline",
			Args:  []string{},
			Stdin: []byte("\n"),
		},
		// Empty line among content.
		{
			Name:  "empty_line_among_content",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcdef\n\nghijkl\n"),
		},
		// Exact width line.
		{
			Name:  "exact_width",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcde\n"),
		},
		// Width 1.
		{
			Name:  "width_1",
			Args:  []string{"-w", "1"},
			Stdin: []byte("abc\n"),
		},
		// Long option --width.
		{
			Name:  "long_width_option",
			Args:  []string{"--width=4"},
			Stdin: []byte("abcdefgh\n"),
		},
		// R3.3: Space at break point is last char before newline.
		{
			Name:  "space_at_break_point",
			Args:  []string{"-w", "6", "-s"},
			Stdin: []byte("ab cd efgh\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
