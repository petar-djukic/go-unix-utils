// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
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
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
