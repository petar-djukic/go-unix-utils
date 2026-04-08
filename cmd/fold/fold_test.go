// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/fold. Implements srd023-fold R4.1, R4.2, R4.3, R4.4.
package main

import (
	"os/exec"
	"strings"
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
		// R4.1: Default width (80 columns), short line passes through unchanged.
		{
			Name:  "default_width_short_line",
			Stdin: []byte("hello world\n"),
		},
		// R4.1: Default width, line exactly 80 chars passes unchanged.
		{
			Name:  "default_width_exact_80",
			Stdin: []byte(strings.Repeat("a", 80) + "\n"),
		},
		// R4.1: Default width, line longer than 80 wraps.
		{
			Name:  "default_width_over_80",
			Stdin: []byte(strings.Repeat("x", 120) + "\n"),
		},
		// R4.1: Default width, line much longer than 80 wraps multiple times.
		{
			Name:  "default_width_multi_wrap",
			Stdin: []byte(strings.Repeat("z", 200) + "\n"),
		},
		// R4.2: Custom -w width wraps correctly.
		{
			Name:  "w_flag_custom_width",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefghij"),
		},
		// R4.2: -w with line exactly at width boundary.
		{
			Name:  "w_flag_exact_boundary",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcde\n"),
		},
		// R4.2: -w with line one char over width.
		{
			Name:  "w_flag_one_over",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcdef\n"),
		},
		// R4.2: -b byte mode wraps at byte positions.
		{
			Name:  "b_flag_byte_mode",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("abcdefgh\n"),
		},
		// R4.2: -b byte mode treats tab as one byte.
		{
			Name:  "b_flag_tab_as_one_byte",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("ab\tcd\n"),
		},
		// R4.2: -s space-break mode breaks at last space within width.
		{
			Name:  "s_flag_space_break",
			Args:  []string{"-w", "8", "-s"},
			Stdin: []byte("hello world test"),
		},
		// R4.2: -s with no space within width falls back to hard break.
		{
			Name:  "s_flag_no_space_hard_break",
			Args:  []string{"-w", "5", "-s"},
			Stdin: []byte("abcdefghij\n"),
		},
		// R4.2: -s -b combined flags.
		{
			Name:  "s_b_combined",
			Args:  []string{"-s", "-b", "-w", "8"},
			Stdin: []byte("hello world test"),
		},
		// R4.2: -s with trailing spaces.
		{
			Name:  "s_flag_trailing_spaces",
			Args:  []string{"-w", "10", "-s"},
			Stdin: []byte("hello     world\n"),
		},
		// R4.3: Tab character in column mode expands to next tab stop.
		{
			Name:  "tab_column_mode",
			Args:  []string{"-w", "9"},
			Stdin: []byte("a\tb\n"),
		},
		// R4.3: Tab at start of line.
		{
			Name:  "tab_at_start",
			Args:  []string{"-w", "10"},
			Stdin: []byte("\thello\n"),
		},
		// R4.3: Multiple tabs.
		{
			Name:  "multiple_tabs",
			Args:  []string{"-w", "16"},
			Stdin: []byte("\t\thi\n"),
		},
		// R4.3: Tab in byte mode counts as 1 byte.
		{
			Name:  "tab_byte_mode",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("\tabcd\n"),
		},
		// R4.3: Backspace character handling in column mode.
		{
			Name:  "backspace_column_mode",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcde\bfghij\n"),
		},
		// R4.3: Backspace in byte mode.
		{
			Name:  "backspace_byte_mode",
			Args:  []string{"-b", "-w", "5"},
			Stdin: []byte("abcde\bfghij\n"),
		},
		// R4.3: Carriage return resets column to 0 in column mode.
		{
			Name:  "carriage_return_column_mode",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abc\rdefgh\n"),
		},
		// R4.3: Carriage return in byte mode counts as 1 byte.
		{
			Name:  "carriage_return_byte_mode",
			Args:  []string{"-b", "-w", "5"},
			Stdin: []byte("abc\rdefgh\n"),
		},
		// R4.3: Tab with -s space mode.
		{
			Name:  "tab_with_s_flag",
			Args:  []string{"-w", "16", "-s"},
			Stdin: []byte("hello\tworld test\n"),
		},
		// R4.4: Empty input produces empty output.
		{
			Name:  "empty_input",
			Stdin: []byte{},
		},
		// R4.4: Single newline.
		{
			Name:  "single_newline",
			Stdin: []byte("\n"),
		},
		// R4.4: Single character without newline.
		{
			Name:  "single_char_no_newline",
			Args:  []string{"-w", "5"},
			Stdin: []byte("a"),
		},
		// R4.4: Single character with newline.
		{
			Name:  "single_char_with_newline",
			Args:  []string{"-w", "5"},
			Stdin: []byte("a\n"),
		},
		// R4.4: Line exactly at width boundary without newline.
		{
			Name:  "exact_width_no_newline",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcde"),
		},
		// R4.4: Line exactly at width boundary with newline.
		{
			Name:  "exact_width_with_newline",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcde\n"),
		},
		// R4.4: Width of 1 wraps every character.
		{
			Name:  "width_one",
			Args:  []string{"-w", "1"},
			Stdin: []byte("abc\n"),
		},
		// R4.4: Multiple lines, some short, some long.
		{
			Name:  "multiple_lines_mixed",
			Args:  []string{"-w", "5"},
			Stdin: []byte("ab\nabcdef\ncd\nabcdefghij\n"),
		},
		// R4.4: Multiple consecutive empty lines.
		{
			Name:  "consecutive_empty_lines",
			Args:  []string{"-w", "10"},
			Stdin: []byte("\n\n\n"),
		},
		// R4.4: No trailing newline on last line, multiple wraps.
		{
			Name:  "no_trailing_newline_multi_wrap",
			Args:  []string{"-w", "3"},
			Stdin: []byte("abcdefghi"),
		},
		// R4.4: -w large value, no wrapping needed.
		{
			Name:  "large_width_no_wrap",
			Args:  []string{"-w", "1000"},
			Stdin: []byte("short line\n"),
		},
		// R4.2: -s -b -w combined on longer input.
		{
			Name:  "all_flags_combined",
			Args:  []string{"-s", "-b", "-w", "8"},
			Stdin: []byte("foo bar baz qux quux\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
