// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/fold against the GNU reference binary (gfold).
// Implements prd023-fold R1.1-R1.4, R2.1-R2.3, R3.1-R3.4, R4.1-R4.4 test coverage.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
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

	// Create a temp file for file-input tests.
	tmpDir := t.TempDir()
	longFile := filepath.Join(tmpDir, "long.txt")
	longLine := strings.Repeat("abcdefghij", 10) + "\n" // 100 chars
	if err := os.WriteFile(longFile, []byte(longLine), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	shortFile := filepath.Join(tmpDir, "short.txt")
	if err := os.WriteFile(shortFile, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R1.1: read from stdin and wrap at default 80 columns.
			Name:  "stdin_default_width",
			Stdin: []byte(strings.Repeat("x", 100) + "\n"),
		},
		{
			// R1.2: lines shorter than width pass through unchanged.
			Name:  "short_line_unchanged",
			Stdin: []byte("short line\n"),
		},
		{
			// R1.2: line exactly 80 chars passes through unchanged.
			Name:  "exact_80_chars",
			Stdin: []byte(strings.Repeat("a", 80) + "\n"),
		},
		{
			// R1.3: line longer than 80 is split.
			Name:  "wrap_at_80",
			Stdin: []byte(strings.Repeat("b", 160) + "\n"),
		},
		{
			// R1.3: wrapping applied repeatedly.
			Name:  "triple_wrap",
			Stdin: []byte(strings.Repeat("c", 200) + "\n"),
		},
		{
			// R1.4: no trailing newline preserved.
			Name:  "no_trailing_newline",
			Stdin: []byte(strings.Repeat("d", 100)),
		},
		{
			// R1.4: trailing newline preserved.
			Name:  "trailing_newline_preserved",
			Stdin: []byte(strings.Repeat("e", 100) + "\n"),
		},
		{
			// R1.1: wrap with -w flag.
			Name:  "custom_width",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefghij"),
		},
		{
			// R1.2: multiple lines.
			Name:  "multiple_lines",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abc\nabcdefgh\nhi\n"),
		},
		{
			// R1.4: '-' means stdin.
			Name:  "dash_stdin",
			Args:  []string{"-w", "10", "-"},
			Stdin: []byte(strings.Repeat("f", 20) + "\n"),
		},
		{
			// R1.2: read from named file.
			Name:    "read_file",
			Args:    []string{shortFile},
			WorkDir: tmpDir,
		},
		{
			// R1.2: read from named file with wrapping.
			Name:    "read_file_wrap",
			Args:    []string{"-w", "20", longFile},
			WorkDir: tmpDir,
		},
		{
			// R1.1: empty input.
			Name:  "empty_input",
			Stdin: []byte(""),
		},
		{
			// R1.2: empty lines pass through.
			Name:  "empty_lines",
			Stdin: []byte("\n\n\n"),
		},
		{
			// R1.3: tab character column counting.
			Name:  "tab_column_counting",
			Args:  []string{"-w", "10"},
			Stdin: []byte("a\tb\n"),
		},
		{
			// R1.3: tab at boundary.
			Name:  "tab_at_boundary",
			Args:  []string{"-w", "8"},
			Stdin: []byte("\thello\n"),
		},
		{
			// R1.1: width of 1.
			Name:  "width_one",
			Args:  []string{"-w", "1"},
			Stdin: []byte("abc\n"),
		},
		{
			// R1.4: file interspersed with stdin via '-'.
			Name:    "file_and_dash",
			Args:    []string{shortFile, "-"},
			Stdin:   []byte("from stdin\n"),
			WorkDir: tmpDir,
		},
		// R2.3: byte counting mode (-b flag).
		{
			// R2.3: -b counts bytes, not columns.
			Name:  "byte_mode_basic",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("abcdefghij"),
		},
		{
			// R2.3: -b with newline-terminated input.
			Name:  "byte_mode_with_newline",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("abcdefghij\n"),
		},
		{
			// R2.3: -b disables tab-stop expansion; tab is 1 byte.
			Name:  "byte_mode_tab_as_one_byte",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("a\tbcd\n"),
		},
		{
			// R2.3: -b with default width (80 bytes).
			Name:  "byte_mode_default_width",
			Args:  []string{"-b"},
			Stdin: []byte(strings.Repeat("x", 100) + "\n"),
		},
		{
			// R2.3: -b with width 1 wraps every byte.
			Name:  "byte_mode_width_one",
			Args:  []string{"-b", "-w", "1"},
			Stdin: []byte("abc\n"),
		},
		{
			// R2.3: -b with multibyte UTF-8 character splits mid-character.
			Name:  "byte_mode_multibyte_utf8",
			Args:  []string{"-b", "-w", "2"},
			Stdin: []byte("a\xc3\xa9b\n"),
		},
		{
			// R2.3: -b combined short flag form -bw4.
			Name:  "byte_mode_combined_flags",
			Args:  []string{"-bw4"},
			Stdin: []byte("abcdefgh\n"),
		},
		{
			// R2.3: -b with long line requiring multiple wraps.
			Name:  "byte_mode_triple_wrap",
			Args:  []string{"-b", "-w", "5"},
			Stdin: []byte(strings.Repeat("z", 17) + "\n"),
		},
		// R3.1-R3.4: space-break mode (-s flag) and interaction with -b.
		{
			// R3.1: -s breaks at last space.
			Name:  "space_break_basic",
			Args:  []string{"-s", "-w", "11"},
			Stdin: []byte("hello world foo bar\n"),
		},
		{
			// R3.2: -s falls back to exact break when no space.
			Name:  "space_break_no_space",
			Args:  []string{"-s", "-w", "5"},
			Stdin: []byte("abcdefghij\n"),
		},
		{
			// R3.4: -b -s combines byte counting with space breaking.
			Name:  "byte_mode_space_break",
			Args:  []string{"-b", "-s", "-w", "11"},
			Stdin: []byte("hello world foo bar\n"),
		},
		{
			// R3.4: -b -s with no space in segment.
			Name:  "byte_mode_space_break_no_space",
			Args:  []string{"-b", "-s", "-w", "5"},
			Stdin: []byte("abcdefghij\n"),
		},
		{
			// R3.3: space is last char on line before newline.
			Name:  "space_break_space_position",
			Args:  []string{"-s", "-w", "6"},
			Stdin: []byte("aa bb cc dd\n"),
		},
		{
			// R3.4: -bs combined short flags.
			Name:  "byte_space_combined_short",
			Args:  []string{"-bs", "-w", "11"},
			Stdin: []byte("hello world foo bar\n"),
		},
		{
			// R3.1: -s breaks at tab (blank character) within width.
			Name:  "space_break_at_tab",
			Args:  []string{"-s", "-w", "10"},
			Stdin: []byte("hello\tworld\tfoo"),
		},
		{
			// R3.3: -s with -b breaks at tab as blank using byte positions.
			Name:  "byte_space_break_at_tab",
			Args:  []string{"-b", "-s", "-w", "10"},
			Stdin: []byte("hello\tworld\tfoo"),
		},
		{
			// R3.1: -s with multiple spaces in segment picks last one.
			Name:  "space_break_multiple_spaces",
			Args:  []string{"-s", "-w", "10"},
			Stdin: []byte("a b c d e f g\n"),
		},
		{
			// R3.2: -s with long word exceeding width hard-wraps.
			Name:  "space_break_long_word",
			Args:  []string{"-s", "-w", "5"},
			Stdin: []byte("ab cdefghij kl\n"),
		},
		{
			// R3.1: -s with trailing spaces.
			Name:  "space_break_trailing_spaces",
			Args:  []string{"-s", "-w", "8"},
			Stdin: []byte("hello   world\n"),
		},
		{
			// R3.3: -s -w custom width.
			Name:  "space_break_custom_width",
			Args:  []string{"-s", "-w", "15"},
			Stdin: []byte("the quick brown fox jumps over\n"),
		},
		{
			// R3.1: -s with only spaces.
			Name:  "space_break_only_spaces",
			Args:  []string{"-s", "-w", "3"},
			Stdin: []byte("          \n"),
		},
		{
			// R3.2: -s with width 1 and spaces.
			Name:  "space_break_width_one",
			Args:  []string{"-s", "-w", "1"},
			Stdin: []byte("a b c\n"),
		},

		// --- R4.1: Differential tests for all flag combinations ---
		{
			// R4.1: -s alone (default width 80) with long input containing spaces.
			Name:  "flag_s_alone_default_width",
			Args:  []string{"-s"},
			Stdin: []byte(strings.Repeat("abcde ", 20) + "\n"),
		},
		{
			// R4.1: -b alone (default width 80).
			Name:  "flag_b_alone",
			Args:  []string{"-b"},
			Stdin: []byte(strings.Repeat("q", 160) + "\n"),
		},
		{
			// R4.1: -w N alone.
			Name:  "flag_w_alone",
			Args:  []string{"-w", "20"},
			Stdin: []byte(strings.Repeat("r", 60) + "\n"),
		},
		{
			// R4.1: -w N -b combined.
			Name:  "flag_w_b_combined",
			Args:  []string{"-w", "10", "-b"},
			Stdin: []byte("a\tb\tc\td\te\tf\n"),
		},
		{
			// R4.1: -w N -s combined.
			Name:  "flag_w_s_combined",
			Args:  []string{"-w", "15", "-s"},
			Stdin: []byte("one two three four five six seven\n"),
		},
		{
			// R4.1: -b -s combined (default width).
			Name:  "flag_b_s_default_width",
			Args:  []string{"-b", "-s"},
			Stdin: []byte(strings.Repeat("word ", 25) + "\n"),
		},
		{
			// R4.1: -w N -b -s all three flags combined.
			Name:  "flag_w_b_s_all_combined",
			Args:  []string{"-w", "12", "-b", "-s"},
			Stdin: []byte("alpha beta gamma delta epsilon\n"),
		},
		{
			// R4.1: all flags combined with short form -bsw.
			Name:  "flag_bsw_short_form",
			Args:  []string{"-bsw12"},
			Stdin: []byte("alpha beta gamma delta epsilon\n"),
		},
		{
			// R4.1: flags in reversed order -s -b -w.
			Name:  "flag_s_b_w_reversed_order",
			Args:  []string{"-s", "-b", "-w", "12"},
			Stdin: []byte("alpha beta gamma delta epsilon\n"),
		},

		// --- R4.2: Edge cases for boundary widths ---
		{
			// R4.2: line one character below fold width (width-1).
			Name:  "boundary_width_minus_one",
			Args:  []string{"-w", "10"},
			Stdin: []byte("123456789\n"),
		},
		{
			// R4.2: line exactly at fold width.
			Name:  "boundary_exact_width",
			Args:  []string{"-w", "10"},
			Stdin: []byte("1234567890\n"),
		},
		{
			// R4.2: line one character above fold width (width+1).
			Name:  "boundary_width_plus_one",
			Args:  []string{"-w", "10"},
			Stdin: []byte("12345678901\n"),
		},
		{
			// R4.2: boundary widths in byte mode (width-1).
			Name:  "boundary_byte_mode_minus_one",
			Args:  []string{"-b", "-w", "10"},
			Stdin: []byte("123456789\n"),
		},
		{
			// R4.2: boundary widths in byte mode (exact).
			Name:  "boundary_byte_mode_exact",
			Args:  []string{"-b", "-w", "10"},
			Stdin: []byte("1234567890\n"),
		},
		{
			// R4.2: boundary widths in byte mode (width+1).
			Name:  "boundary_byte_mode_plus_one",
			Args:  []string{"-b", "-w", "10"},
			Stdin: []byte("12345678901\n"),
		},
		{
			// R4.2: boundary widths with -s, space at exactly the width.
			Name:  "boundary_space_at_width",
			Args:  []string{"-s", "-w", "10"},
			Stdin: []byte("1234567890 abc\n"),
		},
		{
			// R4.2: boundary widths with -s, space at width-1.
			Name:  "boundary_space_at_width_minus_one",
			Args:  []string{"-s", "-w", "10"},
			Stdin: []byte("123456789 abcdef\n"),
		},
		{
			// R4.2: boundary widths with -s, space at width+1 (no space within width).
			Name:  "boundary_space_at_width_plus_one",
			Args:  []string{"-s", "-w", "10"},
			Stdin: []byte("12345678901 ab\n"),
		},
		{
			// R4.2: boundary with -b -s, space exactly at byte width.
			Name:  "boundary_byte_space_at_width",
			Args:  []string{"-b", "-s", "-w", "10"},
			Stdin: []byte("1234567890 abc\n"),
		},
		{
			// R4.2: exact multiple of width (no remainder).
			Name:  "boundary_exact_multiple",
			Args:  []string{"-w", "5"},
			Stdin: []byte("1234512345\n"),
		},
		{
			// R4.2: exact multiple of width minus one.
			Name:  "boundary_exact_multiple_minus_one",
			Args:  []string{"-w", "5"},
			Stdin: []byte("123451234\n"),
		},

		// --- R4.3: Edge cases for empty, single-char, no-newline, whitespace ---
		{
			// R4.3: empty stdin (already tested above, repeated for clarity).
			Name:  "edge_empty_stdin",
			Stdin: []byte{},
		},
		{
			// R4.3: single character with newline.
			Name:  "edge_single_char_with_newline",
			Stdin: []byte("x\n"),
		},
		{
			// R4.3: single character without newline.
			Name:  "edge_single_char_no_newline",
			Stdin: []byte("x"),
		},
		{
			// R4.3: input with no newlines at all (long).
			Name:  "edge_no_newline_long",
			Args:  []string{"-w", "10"},
			Stdin: []byte(strings.Repeat("a", 25)),
		},
		{
			// R4.3: input consisting entirely of spaces.
			Name:  "edge_all_spaces",
			Args:  []string{"-w", "5"},
			Stdin: []byte("          \n"),
		},
		{
			// R4.3: input consisting entirely of tabs.
			Name:  "edge_all_tabs",
			Args:  []string{"-w", "10"},
			Stdin: []byte("\t\t\t\n"),
		},
		{
			// R4.3: input of spaces with -s flag.
			Name:  "edge_all_spaces_with_s",
			Args:  []string{"-s", "-w", "5"},
			Stdin: []byte("          \n"),
		},
		{
			// R4.3: input of tabs with -b flag (tabs count as 1 byte each).
			Name:  "edge_all_tabs_byte_mode",
			Args:  []string{"-b", "-w", "3"},
			Stdin: []byte("\t\t\t\t\t\n"),
		},
		{
			// R4.3: single newline only.
			Name:  "edge_single_newline",
			Stdin: []byte("\n"),
		},
		{
			// R4.3: multiple empty lines.
			Name:  "edge_multiple_empty_lines",
			Stdin: []byte("\n\n\n\n"),
		},
		{
			// R4.3: empty input with -s flag.
			Name:  "edge_empty_with_s",
			Args:  []string{"-s"},
			Stdin: []byte{},
		},
		{
			// R4.3: empty input with -b flag.
			Name:  "edge_empty_with_b",
			Args:  []string{"-b"},
			Stdin: []byte{},
		},
		{
			// R4.3: spaces with -b -s combined.
			Name:  "edge_spaces_b_s",
			Args:  []string{"-b", "-s", "-w", "3"},
			Stdin: []byte("     \n"),
		},

		// --- R4.4: Multibyte UTF-8 characters near fold boundaries ---
		{
			// R4.4: multibyte UTF-8 (2-byte é) near width boundary in column mode.
			// In LC_ALL=C, each byte counts as one column.
			Name:  "utf8_two_byte_near_boundary",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abc\xc3\xa9d\n"),
		},
		{
			// R4.4: multibyte UTF-8 in byte mode splits mid-character.
			Name:  "utf8_byte_mode_split",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("ab\xc3\xa9cd\n"),
		},
		{
			// R4.4: 3-byte UTF-8 character (€ = 0xe2 0x82 0xac) near boundary.
			Name:  "utf8_three_byte_near_boundary",
			Args:  []string{"-w", "4"},
			Stdin: []byte("ab\xe2\x82\xaccd\n"),
		},
		{
			// R4.4: 3-byte UTF-8 in byte mode with width splitting mid-character.
			Name:  "utf8_three_byte_byte_mode",
			Args:  []string{"-b", "-w", "3"},
			Stdin: []byte("a\xe2\x82\xacb\n"),
		},
		{
			// R4.4: 4-byte UTF-8 character (𝄞 = 0xf0 0x9d 0x84 0x9e) near boundary.
			Name:  "utf8_four_byte_near_boundary",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abc\xf0\x9d\x84\x9eef\n"),
		},
		{
			// R4.4: 4-byte UTF-8 in byte mode.
			Name:  "utf8_four_byte_byte_mode",
			Args:  []string{"-b", "-w", "5"},
			Stdin: []byte("abc\xf0\x9d\x84\x9eef\n"),
		},
		{
			// R4.4: mixed ASCII and multibyte, word-break mode.
			Name:  "utf8_mixed_space_break",
			Args:  []string{"-s", "-w", "10"},
			Stdin: []byte("hello \xc3\xa9\xc3\xa0\xc3\xbc world\n"),
		},
		{
			// R4.4: mixed ASCII and multibyte with -b -s.
			Name:  "utf8_mixed_byte_space_break",
			Args:  []string{"-b", "-s", "-w", "10"},
			Stdin: []byte("hello \xc3\xa9\xc3\xa0\xc3\xbc world\n"),
		},
		{
			// R4.4: multibyte at exact fold boundary in column mode.
			Name:  "utf8_at_exact_boundary",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcd\xc3\xa9fgh\n"),
		},
		{
			// R4.4: string of multibyte characters only.
			Name:  "utf8_all_multibyte",
			Args:  []string{"-w", "6"},
			Stdin: []byte("\xc3\xa9\xc3\xa0\xc3\xbc\xc3\xa9\xc3\xa0\xc3\xbc\xc3\xa9\n"),
		},
		{
			// R4.4: multibyte characters in byte mode at width 1.
			Name:  "utf8_byte_mode_width_one",
			Args:  []string{"-b", "-w", "1"},
			Stdin: []byte("\xc3\xa9\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
