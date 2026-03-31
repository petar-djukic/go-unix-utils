// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/fold against gfold (GNU coreutils).
//
// Covers prd023-fold R1.1-R1.4, R2.1-R2.3, R3.1-R3.4, R4.1-R4.4.
package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeFoldName replaces the gfold/fold binary name prefix in stderr
// so that error messages from the reference and Go binaries can be compared.
var normalizeFoldName testutils.NormalizeFunc = func(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gfold:"), []byte("fold:"))
}

// normalizeFoldErrors normalizes binary name and OS error message capitalization
// differences between GNU (C strerror) and Go (syscall.Errno.Error).
var normalizeFoldErrors = testutils.ComposeNormalizers(
	normalizeFoldName,
	func(data []byte) []byte {
		// Go returns lowercase "no such file or directory"; GNU returns capitalized.
		data = bytes.ReplaceAll(data, []byte("No such file or directory"), []byte("no such file or directory"))
		data = bytes.ReplaceAll(data, []byte("Permission denied"), []byte("permission denied"))
		data = bytes.ReplaceAll(data, []byte("Is a directory"), []byte("is a directory"))
		return data
	},
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfold")
	if err != nil {
		t.Skip("reference binary gfold not in PATH")
	}

	tests := []testutils.DiffTest{
		// --- R1.1: read stdin when no files given, wrap to default 80 ---
		{
			Name:  "r1_1_stdin_default_width",
			Args:  []string{},
			Stdin: []byte(strings.Repeat("A", 90) + "\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r1_1_stdin_explicit_width",
			Args:  []string{"-w", "10"},
			Stdin: []byte(strings.Repeat("z", 25) + "\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R1.2: lines within width pass through unchanged ---
		{
			Name:  "r1_2_short_line_unchanged",
			Args:  []string{"-w", "20"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r1_2_exact_width_unchanged",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcde\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r1_2_empty_line_unchanged",
			Args:  []string{"-w", "10"},
			Stdin: []byte("\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R1.3: lines exceeding width are split repeatedly ---
		{
			Name:  "r1_3_split_once",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcdefgh\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r1_3_split_multiple",
			Args:  []string{"-w", "3"},
			Stdin: []byte("abcdefghijkl\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r1_3_split_exact_multiple",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefgh\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R1.4: final segment preserves trailing newline presence ---
		{
			Name:  "r1_4_with_trailing_newline",
			Args:  []string{"-w", "3"},
			Stdin: []byte("abcde\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r1_4_without_trailing_newline",
			Args:  []string{"-w", "3"},
			Stdin: []byte("abcde"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r1_4_multiline_last_no_newline",
			Args:  []string{"-w", "5"},
			Stdin: []byte("hello\nworldXXXXX"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R2.1: -w N sets max line width ---
		{
			Name:  "r2_1_w4_wraps_at_4",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefghij"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_1_w1_wraps_every_char",
			Args:  []string{"-w", "1"},
			Stdin: []byte("abcd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_1_w_large_no_wrap",
			Args:  []string{"-w", "200"},
			Stdin: []byte(strings.Repeat("x", 100) + "\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:      "r2_1_w0_error",
			Args:      []string{"-w", "0"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeFoldName},
		},
		{
			Name:      "r2_1_w_negative_error",
			Args:      []string{"-w", "-1"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeFoldName},
		},
		{
			Name:      "r2_1_w_nonnumeric_error",
			Args:      []string{"-w", "abc"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeFoldName},
		},

		// --- R2.2: tab stop column counting (every 8 columns) ---
		{
			Name:  "r2_2_tab_expands_to_8",
			Args:  []string{"-w", "8"},
			Stdin: []byte("\tx\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_2_tab_after_1_char",
			Args:  []string{"-w", "9"},
			Stdin: []byte("a\tbcde\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_2_tab_after_7_chars",
			Args:  []string{"-w", "10"},
			Stdin: []byte("1234567\t89\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_2_multiple_tabs",
			Args:  []string{"-w", "16"},
			Stdin: []byte("\t\t\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_2_tab_causes_wrap",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abc\tdef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_2_control_char_one_col",
			Args:  []string{"-w", "5"},
			Stdin: append([]byte("abcd"), 0x01, 0x02, '\n'),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_2_backspace_decrements",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcde\b\bfghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_2_cr_resets_col",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcde\rfghij\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R2.3: -b counts bytes, tabs count as 1 byte ---
		{
			Name:  "r2_3_tab_counts_as_1_byte",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("\t\t\t\t\tX\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_3_byte_mode_simple",
			Args:  []string{"-b", "-w", "5"},
			Stdin: []byte("1234567890\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_3_byte_vs_column_tab",
			Args:  []string{"-b", "-w", "3"},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_3_backspace_counts_as_1_byte",
			Args:  []string{"-b", "-w", "5"},
			Stdin: []byte("abc\b\bdefghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_3_cr_counts_as_1_byte",
			Args:  []string{"-b", "-w", "5"},
			Stdin: []byte("abcd\refghij\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R3.1: -s breaks at last space at or before wrap column ---
		{
			Name:  "r3_1_break_at_last_space",
			Args:  []string{"-s", "-w", "11"},
			Stdin: []byte("hello world foo bar\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_1_multiple_spaces_picks_last",
			Args:  []string{"-s", "-w", "15"},
			Stdin: []byte("one two three four five\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_1_space_exactly_at_width",
			Args:  []string{"-s", "-w", "6"},
			Stdin: []byte("abcde fghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_1_trailing_spaces",
			Args:  []string{"-s", "-w", "8"},
			Stdin: []byte("abc def  ghi\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R3.2: no space within width falls back to hard break ---
		{
			Name:  "r3_2_no_space_hard_break",
			Args:  []string{"-s", "-w", "5"},
			Stdin: []byte("abcdefghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_2_no_space_long_word",
			Args:  []string{"-s", "-w", "4"},
			Stdin: []byte("abcdefghijklmnop\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_2_space_after_hard_break",
			Args:  []string{"-s", "-w", "5"},
			Stdin: []byte("abcdefg hijk\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R3.3: space written as last char before newline, next line starts after space ---
		{
			Name:  "r3_3_space_is_last_char_before_newline",
			Args:  []string{"-s", "-w", "10"},
			Stdin: []byte("hello world goodbye now\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_3_consecutive_spaces",
			Args:  []string{"-s", "-w", "8"},
			Stdin: []byte("aa bb cc dd ee\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_3_space_at_start_of_segment",
			Args:  []string{"-s", "-w", "5"},
			Stdin: []byte("abc  defgh\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R3.4: -s compatible with -b ---
		{
			Name:  "r3_4_bs_simple",
			Args:  []string{"-b", "-s", "-w", "8"},
			Stdin: []byte("hello world goodbye\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_4_bs_tab_not_expanded",
			Args:  []string{"-b", "-s", "-w", "6"},
			Stdin: []byte("ab\tcd efgh\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_4_bs_no_space_fallback",
			Args:  []string{"-b", "-s", "-w", "5"},
			Stdin: []byte("abcdefghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_4_bs_long_mixed",
			Args:  []string{"-b", "-s", "-w", "10"},
			Stdin: []byte("abcdefghijklmnop qrs tuv\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R4.1: exit 0 on success ---
		{
			Name:  "r4_1_exit_0_success_stdin",
			Args:  []string{"-w", "10"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r4_1_exit_0_empty_input",
			Args:  []string{},
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},

		// --- R4.2: exit 1 on file open error, continue processing ---
		{
			Name:      "r4_2_nonexistent_file",
			Args:      []string{"/nonexistent/path/to/fold_test_file"},
			Stdin:     nil,
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeFoldErrors},
		},
		{
			Name:      "r4_2_mixed_stdin_and_bad_file",
			Args:      []string{"-", "/nonexistent/fold_r4_2_test"},
			Stdin:     []byte("good input\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeFoldErrors},
		},
		{
			Name:      "r4_2_bad_file_then_stdin",
			Args:      []string{"/nonexistent/fold_r4_2_first", "-"},
			Stdin:     []byte("still works\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeFoldErrors},
		},

		// --- default folding (no flags) ---
		{
			Name:  "default_short_line",
			Args:  []string{},
			Stdin: []byte("short line\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "default_exactly_80",
			Args:  []string{},
			Stdin: []byte(strings.Repeat("x", 80) + "\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "default_over_80",
			Args:  []string{},
			Stdin: []byte(strings.Repeat("a", 100) + "\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "default_multiple_lines",
			Args:  []string{},
			Stdin: []byte("hello\nworld\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R4.2: -w flag with various widths ---
		{
			Name:  "w4_wrap_10_chars",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefghij"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "w1_single_char_width",
			Args:  []string{"-w", "1"},
			Stdin: []byte("abc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "w20_short_input",
			Args:  []string{"-w", "20"},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "w5_exact_multiple",
			Args:  []string{"-w", "5"},
			Stdin: []byte("1234512345\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R4.2: -s flag for space-break mode ---
		{
			Name:  "s_w11_break_at_space",
			Args:  []string{"-w", "11", "-s"},
			Stdin: []byte("hello world foo\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "s_w4_no_space_fallback",
			Args:  []string{"-w", "4", "-s"},
			Stdin: []byte("abcdefgh\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "s_w10_multiple_spaces",
			Args:  []string{"-w", "10", "-s"},
			Stdin: []byte("one two three four five\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "s_w6_space_at_boundary",
			Args:  []string{"-w", "6", "-s"},
			Stdin: []byte("abcde fghij\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R4.2: -b flag for byte-counting mode ---
		{
			Name:  "b_w4_simple",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("abcde"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "b_w4_with_tab",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("a\tbcde\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R4.3: flag combinations ---
		{
			Name:  "bs_w8_byte_space",
			Args:  []string{"-b", "-s", "-w", "8"},
			Stdin: []byte("hello world goodbye\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "bw5_byte_width",
			Args:  []string{"-b", "-w", "5"},
			Stdin: []byte("1234567890\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "sw8_space_width",
			Args:  []string{"-s", "-w", "8"},
			Stdin: []byte("aa bb cc dd ee ff\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "bsw6_all_flags",
			Args:  []string{"-b", "-s", "-w", "6"},
			Stdin: []byte("ab cd ef gh ij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "bsw10_long_word",
			Args:  []string{"-b", "-s", "-w", "10"},
			Stdin: []byte("abcdefghijklmnop qrs\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R4.4: edge cases ---
		{
			Name:  "edge_empty_input",
			Args:  []string{},
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "edge_single_char",
			Args:  []string{"-w", "5"},
			Stdin: []byte("x\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "edge_lines_shorter_than_width",
			Args:  []string{"-w", "20"},
			Stdin: []byte("short\nlines\nhere\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "edge_tabs_column_counting",
			Args:  []string{"-w", "9"},
			Stdin: []byte("a\tbcde\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "edge_tab_at_col_boundary",
			Args:  []string{"-w", "8"},
			Stdin: []byte("\tabcdefgh\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "edge_backspace",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abc\b\bdefghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "edge_carriage_return",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcde\rfghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "edge_no_trailing_newline",
			Args:  []string{"-w", "3"},
			Stdin: []byte("abcdef"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "edge_only_newlines",
			Args:  []string{"-w", "5"},
			Stdin: []byte("\n\n\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "edge_single_char_no_newline",
			Args:  []string{"-w", "1"},
			Stdin: []byte("a"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "edge_backspace_byte_mode",
			Args:  []string{"-b", "-w", "5"},
			Stdin: []byte("abc\b\bdefghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "edge_cr_byte_mode",
			Args:  []string{"-b", "-w", "5"},
			Stdin: []byte("abc\rdefgh\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "edge_tabs_byte_mode",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("\t\tabcd\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
