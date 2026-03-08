// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for the fold utility,
// comparing output against the GNU reference binary (gfold).
//
// Tests trace to prd023-fold R1, R2, R3, R4.
package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfold")
	if err != nil {
		t.Skipf("reference binary gfold not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.2: short line passes through unchanged at default width 80.
		{
			Name:  "default_width_80_short",
			Args:  []string{},
			Stdin: []byte("short line\n"),
		},
		// R1.2: line of exactly 80 chars passes through unchanged.
		{
			Name:  "exactly_80_chars",
			Args:  []string{},
			Stdin: []byte(strings.Repeat("x", 80) + "\n"),
		},
		// R1.3: line longer than 80 chars is wrapped.
		{
			Name:  "wrap_at_80",
			Args:  []string{},
			Stdin: []byte(strings.Repeat("a", 100) + "\n"),
		},
		// R1.1, R1.3: -w 4 wraps at 4 columns.
		{
			Name:  "wrap_at_width_4",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefghij"),
		},
		// R1.4: trailing newline preserved after wrapping.
		{
			Name:  "wrap_with_trailing_newline",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefghij\n"),
		},
		// R1.2: line shorter than width unchanged.
		{
			Name:  "short_line_unchanged",
			Args:  []string{"-w", "20"},
			Stdin: []byte("hello\n"),
		},
		// R2.2: tab expansion for column counting.
		{
			Name:  "tab_column_expansion",
			Args:  []string{"-w", "9"},
			Stdin: []byte("a\tbcde\n"),
		},
		// R2.3: -b counts bytes, not columns.
		{
			Name:  "byte_mode",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("abcde"),
		},
		// R2.3: -b with tab (tab counts as 1 byte).
		{
			Name:  "byte_mode_tab",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("a\tbcde\n"),
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
		// R3.3: space at break point is last char of segment.
		{
			Name:  "space_break_space_position",
			Args:  []string{"-w", "6", "-s"},
			Stdin: []byte("abc def ghi\n"),
		},
		// R3.4: -s with -b.
		{
			Name:  "space_break_byte_mode",
			Args:  []string{"-w", "6", "-s", "-b"},
			Stdin: []byte("abc def ghi\n"),
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
		// Multiple lines.
		{
			Name:  "multiple_lines",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcdefgh\n12345678\n"),
		},
		// Very long line.
		{
			Name:  "very_long_line",
			Args:  []string{"-w", "10"},
			Stdin: []byte(strings.Repeat("x", 250) + "\n"),
		},
		// Multiple empty lines.
		{
			Name:  "multiple_empty_lines",
			Args:  []string{},
			Stdin: []byte("\n\n\n"),
		},
		// Width 1.
		{
			Name:  "width_1",
			Args:  []string{"-w", "1"},
			Stdin: []byte("abc\n"),
		},
		// -s with multiple spaces.
		{
			Name:  "space_break_multiple_spaces",
			Args:  []string{"-w", "10", "-s"},
			Stdin: []byte("one two three four five\n"),
		},
		// Tab at various positions.
		{
			Name:  "tab_at_start",
			Args:  []string{"-w", "10"},
			Stdin: []byte("\tabcdef\n"),
		},
		// Backspace character.
		{
			Name:  "backspace_char",
			Args:  []string{"-w", "4"},
			Stdin: []byte("ab\bcd\n"),
		},
		// No trailing newline at end of input.
		{
			Name:  "no_trailing_newline",
			Args:  []string{"-w", "3"},
			Stdin: []byte("abcdef"),
		},
		// -w with = syntax.
		{
			Name:  "width_equals_syntax",
			Args:  []string{"--width=4"},
			Stdin: []byte("abcdefgh\n"),
		},
		// Combined -bs flags.
		{
			Name:  "combined_bs_flags",
			Args:  []string{"-bs", "-w", "5"},
			Stdin: []byte("ab cd efgh\n"),
		},
		// Space at exact width boundary with -s.
		{
			Name:  "space_at_width_boundary",
			Args:  []string{"-w", "5", "-s"},
			Stdin: []byte("abcde fghij\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
