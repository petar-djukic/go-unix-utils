// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfold")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		{
			Name:  "R1.1_default_width_80",
			Stdin: []byte("abcdefghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.2_short_line_unchanged",
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.2_exact_width_unchanged",
			Args:  []string{"-w", "10"},
			Stdin: []byte("abcdefghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.3_wrap_at_width",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.3_repeated_wrapping",
			Args:  []string{"-w", "3"},
			Stdin: []byte("abcdefghijklm\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.4_no_trailing_newline",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefghij"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.4_trailing_newline_preserved",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefgh\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.1_multiple_lines",
			Stdin: []byte("short\nthis line is a bit longer than the default eighty column width and should be wrapped accordingly by the fold utility\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.1_empty_input",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.2_empty_line",
			Stdin: []byte("\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.3_wrap_width_1",
			Args:  []string{"-w", "1"},
			Stdin: []byte("abc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.1_tab_column_handling",
			Args:  []string{"-w", "9"},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.1_tab_at_boundary",
			Args:  []string{"-w", "8"},
			Stdin: []byte("\txxxxxxxx\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.1_byte_mode_basic",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("abcdefghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.1_byte_mode_tab_no_expansion",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("ab\tcd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.3_single_char_line",
			Args:  []string{"-w", "5"},
			Stdin: []byte("a\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.1_stdin_default",
			Stdin: []byte("line one\nline two\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.4_multiple_lines_no_trailing_newline",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcdef\nghijk"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.3_backspace_handling",
			Args:  []string{"-w", "5"},
			Stdin: []byte("ab\bcdefgh\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.3_carriage_return_handling",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abc\rdefgh\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.1_width_flag_inline",
			Args:  []string{"-w5"},
			Stdin: []byte("abcdefghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.1_width_flag_separate",
			Args:  []string{"-w", "10"},
			Stdin: []byte("abcdefghijklmno\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.1_width_1",
			Args:  []string{"-w", "1"},
			Stdin: []byte("abcd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.1_width_large",
			Args:  []string{"-w", "200"},
			Stdin: []byte("short line\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.2_tab_at_col_0",
			Args:  []string{"-w", "8"},
			Stdin: []byte("\thello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.2_tab_mid_column",
			Args:  []string{"-w", "10"},
			Stdin: []byte("abc\tdefgh\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.2_multiple_tabs",
			Args:  []string{"-w", "20"},
			Stdin: []byte("a\tb\tc\td\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.2_tab_causes_wrap",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abc\tdef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.2_control_char_one_column",
			Args:  []string{"-w", "5"},
			Stdin: []byte("ab\x01\x02cdef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.2_tab_near_boundary",
			Args:  []string{"-w", "9"},
			Stdin: []byte("1234567\tx\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.2_consecutive_tabs",
			Args:  []string{"-w", "16"},
			Stdin: []byte("\t\tabcdef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.3_byte_mode_tab_as_one_byte",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("\tabcdef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.3_byte_mode_multibyte_char",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("\xc3\xa9\xc3\xa9\xc3\xa9\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.3_byte_mode_control_chars",
			Args:  []string{"-b", "-w", "3"},
			Stdin: []byte("\x01\x02\x03\x04\x05\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.3_byte_mode_combined_bs",
			Args:  []string{"-bs", "-w", "6"},
			Stdin: []byte("hello world test\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.3_byte_mode_width_inline",
			Args:  []string{"-bw4"},
			Stdin: []byte("abcdefghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.1_space_break_basic",
			Args:  []string{"-s", "-w", "11"},
			Stdin: []byte("hello world foo bar\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.1_space_break_multiple_spaces",
			Args:  []string{"-s", "-w", "10"},
			Stdin: []byte("aa bb cc dd ee ff gg\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.1_space_break_exact_boundary",
			Args:  []string{"-s", "-w", "5"},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.1_space_break_short_line",
			Args:  []string{"-s", "-w", "20"},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.2_no_space_fallback",
			Args:  []string{"-s", "-w", "5"},
			Stdin: []byte("abcdefghijklmno\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.2_no_space_then_space",
			Args:  []string{"-s", "-w", "5"},
			Stdin: []byte("abcdefgh ij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.3_space_included_before_newline",
			Args:  []string{"-s", "-w", "6"},
			Stdin: []byte("aa bb cc dd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.3_trailing_spaces",
			Args:  []string{"-s", "-w", "8"},
			Stdin: []byte("abc def ghi jkl\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.3_space_at_width",
			Args:  []string{"-s", "-w", "5"},
			Stdin: []byte("abcd efgh\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.4_space_break_byte_mode",
			Args:  []string{"-bs", "-w", "8"},
			Stdin: []byte("hello world foo bar\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.4_space_break_byte_mode_no_space",
			Args:  []string{"-b", "-s", "-w", "5"},
			Stdin: []byte("abcdefghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.4_space_break_byte_mode_multibyte",
			Args:  []string{"-bs", "-w", "6"},
			Stdin: []byte("\xc3\xa9\xc3\xa9 \xc3\xa9\xc3\xa9 \xc3\xa9\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.1_space_break_no_trailing_newline",
			Args:  []string{"-s", "-w", "6"},
			Stdin: []byte("aa bb cc dd"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.1_space_break_multiple_lines",
			Args:  []string{"-s", "-w", "10"},
			Stdin: []byte("hello world foo\nbar baz qux quux\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.1_space_break_with_tab",
			Args:  []string{"-s", "-w", "12"},
			Stdin: []byte("hello\tworld test\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffExitCodes(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfold")
	if err != nil {
		t.Skip("reference binary not found")
	}

	dir := t.TempDir()
	writeFixture(t, dir, "valid.txt", "hello world\n")

	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	tests := []testutils.DiffTest{
		{
			Name:    "R4.1_success_exit_0",
			Args:    []string{"valid.txt"},
			WorkDir: dir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:      "R4.2_nonexistent_file_exit_1",
			Args:      []string{"nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
			Env:       []string{"LC_ALL=C"},
		},
		{
			Name:      "R4.2_nonexistent_with_valid_continues",
			Args:      []string{"nonexistent.txt", "valid.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
			Env:       []string{"LC_ALL=C"},
		},
		{
			Name:      "R4.2_valid_then_nonexistent",
			Args:      []string{"valid.txt", "nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
			Env:       []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(dir+"/"+name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
