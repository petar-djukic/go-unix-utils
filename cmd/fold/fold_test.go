// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/fold against gfold reference binary.
// Implements: prd023-fold AC1-AC5
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
		// R1.1: Default wrap at 80 columns from stdin.
		{
			Name:  "default_80_col",
			Stdin: []byte(strings.Repeat("a", 100) + "\n"),
		},
		// R1.2: Line at exactly 80 chars passes through unchanged.
		{
			Name:  "exactly_80_unchanged",
			Stdin: []byte(strings.Repeat("x", 80) + "\n"),
		},
		// R1.2: Short line passes through unchanged.
		{
			Name:  "short_line_unchanged",
			Stdin: []byte("hello world\n"),
		},
		// R1.3: Repeated wrapping of very long line.
		{
			Name:  "wrap_very_long_line",
			Args:  []string{"-w", "10"},
			Stdin: []byte(strings.Repeat("b", 35) + "\n"),
		},
		// R1.4: No trailing newline preserved.
		{
			Name:  "no_trailing_newline",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefghij"),
		},
		// R1.4: Trailing newline preserved.
		{
			Name:  "trailing_newline_preserved",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefghij\n"),
		},
		// R2.1: -w flag sets custom width.
		{
			Name:  "custom_width_4",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefghij\n"),
		},
		// R2.2: Tab expansion in column mode.
		{
			Name:  "tab_expansion_col_mode",
			Args:  []string{"-w", "9"},
			Stdin: []byte("a\tb\n"),
		},
		// R2.2: Tab at start of line.
		{
			Name:  "tab_at_start",
			Args:  []string{"-w", "10"},
			Stdin: []byte("\thello\n"),
		},
		// R2.3: Byte mode with -b.
		{
			Name:  "byte_mode",
			Args:  []string{"-b", "-w", "5"},
			Stdin: []byte("abcdefghij\n"),
		},
		// R2.3: Byte mode tabs count as 1 byte.
		{
			Name:  "byte_mode_tab",
			Args:  []string{"-b", "-w", "5"},
			Stdin: []byte("ab\tcdefgh\n"),
		},
		// R3.1: Space-break mode.
		{
			Name:  "space_break",
			Args:  []string{"-w", "11", "-s"},
			Stdin: []byte("hello world foo bar\n"),
		},
		// R3.2: Space-break with no spaces falls back to hard break.
		{
			Name:  "space_break_no_spaces",
			Args:  []string{"-w", "5", "-s"},
			Stdin: []byte("abcdefghij\n"),
		},
		// R3.3: Space is included at end of line before break.
		{
			Name:  "space_at_break_point",
			Args:  []string{"-w", "6", "-s"},
			Stdin: []byte("abc def ghi\n"),
		},
		// R3.4: Space-break with byte mode.
		{
			Name:  "space_break_byte_mode",
			Args:  []string{"-w", "10", "-s", "-b"},
			Stdin: []byte("hello world foo bar baz\n"),
		},
		// Combined flags: -bsw.
		{
			Name:  "combined_bsw",
			Args:  []string{"-bsw", "8"},
			Stdin: []byte("the quick brown fox jumps\n"),
		},
		// Empty input.
		{
			Name:  "empty_input",
			Stdin: []byte(""),
		},
		// Empty line.
		{
			Name:  "empty_line",
			Stdin: []byte("\n"),
		},
		// Multiple lines.
		{
			Name:  "multiple_lines",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcdef\n123456789\nhi\n"),
		},
		// Width 1.
		{
			Name:  "width_1",
			Args:  []string{"-w", "1"},
			Stdin: []byte("abcd\n"),
		},
		// Backspace handling in column mode.
		{
			Name:  "backspace_col_mode",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcde\bfghij\n"),
		},
		// Carriage return in column mode.
		{
			Name:  "carriage_return",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abc\rdefgh\n"),
		},
		// Multiple tabs.
		{
			Name:  "multiple_tabs",
			Args:  []string{"-w", "20"},
			Stdin: []byte("a\tb\tc\td\n"),
		},
		// -s with only spaces.
		{
			Name:  "space_break_all_spaces",
			Args:  []string{"-w", "3", "-s"},
			Stdin: []byte("     \n"),
		},
		// Stdin via - argument.
		{
			Name:  "stdin_via_dash",
			Args:  []string{"-w", "5", "-"},
			Stdin: []byte("abcdefgh\n"),
		},
		// Long space-separated words.
		{
			Name:  "space_break_long_word",
			Args:  []string{"-w", "5", "-s"},
			Stdin: []byte("ab cdefghij\n"),
		},
		// R3.1: Break at last space before wrap column with multiple spaces.
		{
			Name:  "space_break_multiple_spaces",
			Args:  []string{"-w", "10", "-s"},
			Stdin: []byte("one two three four five six\n"),
		},
		// R3.2: Fallback to hard break when first word exceeds width.
		{
			Name:  "space_break_first_word_exceeds",
			Args:  []string{"-w", "3", "-s"},
			Stdin: []byte("abcdef gh\n"),
		},
		// R3.3: Space is last char on output line before inserted newline.
		{
			Name:  "space_break_space_retained",
			Args:  []string{"-w", "5", "-s"},
			Stdin: []byte("ab cd efgh\n"),
		},
		// R3.4: Space-break combined with byte mode on tab-containing input.
		{
			Name:  "space_break_byte_mode_tab",
			Args:  []string{"-w", "8", "-s", "-b"},
			Stdin: []byte("abc\t def ghi\n"),
		},
		// R3.1: Break at space at exactly the wrap column.
		{
			Name:  "space_break_at_exact_column",
			Args:  []string{"-w", "5", "-s"},
			Stdin: []byte("abcd efgh\n"),
		},
		// R3.2: No space in segment, repeated hard wraps.
		{
			Name:  "space_break_no_space_repeated",
			Args:  []string{"-w", "4", "-s"},
			Stdin: []byte("abcdefghijkl\n"),
		},
		// R3.4: -s -b with no trailing newline.
		{
			Name:  "space_break_byte_no_newline",
			Args:  []string{"-w", "6", "-s", "-b"},
			Stdin: []byte("ab cd efgh ij"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
