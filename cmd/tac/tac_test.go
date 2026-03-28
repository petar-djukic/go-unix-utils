// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd021-tac R4.1, R4.2, R4.3.
// R4.1: compare Go tac against gtac byte-for-byte via RunDiffTests.
// R4.2: cover default reversal, stdin, multi-file, -s, -b, -r, no trailing newline.
// R4.3: LC_ALL=C set by default in RunDiffTests.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtac")
	if err != nil {
		t.Skip("reference binary gtac not in PATH")
	}

	tests := []testutils.DiffTest{
		// R4.2: default reversal — lines reversed, trailing newline preserved (R1.1, R1.2).
		{
			Name:  "default_reversal",
			Stdin: []byte("alpha\nbeta\ngamma\n"),
		},

		// R4.2: no trailing newline — last line (now first) has no newline (R1.2).
		{
			Name:  "no_trailing_newline",
			Stdin: []byte("a\nb\nc"),
		},

		// R4.2: single line input reverses to itself (R1.1).
		{
			Name:  "single_line",
			Stdin: []byte("only\n"),
		},

		// R4.2: empty input — produces no output.
		{
			Name:  "empty_input",
			Stdin: []byte(""),
		},

		// R4.2: stdin via "-" argument (R1.3).
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("x\ny\nz\n"),
		},

		// R4.2: -s with custom separator (R2.1).
		{
			Name:  "custom_separator",
			Args:  []string{"-s", ":"},
			Stdin: []byte("a:b:c:"),
		},

		// R4.2: -b flag places separator before record (R2.2).
		{
			Name:  "before_flag",
			Args:  []string{"-b", "-s", ":"},
			Stdin: []byte(":a:b:c"),
		},

		// R4.2: -r flag interprets separator as regex (R2.3).
		{
			Name:  "regex_separator",
			Args:  []string{"-r", "-s", "[:|]"},
			Stdin: []byte("a:b|c:"),
		},

		// R4.2: -r -b combined (R2.3, R2.2).
		{
			Name:  "regex_before",
			Args:  []string{"-r", "-b", "-s", "[:|]"},
			Stdin: []byte(":a|b:c"),
		},

		// R4.2: multi-character separator (R2.1).
		{
			Name:  "multi_char_separator",
			Args:  []string{"-s", "::"},
			Stdin: []byte("first::second::third::"),
		},

		// R4.2: -b with default newline separator (R2.2).
		{
			Name:  "before_default_separator",
			Args:  []string{"-b"},
			Stdin: []byte("\nfoo\nbar\nbaz"),
		},

		// Edge: single character input with no newline.
		{
			Name:  "single_char_no_newline",
			Stdin: []byte("x"),
		},

		// Edge: multiple consecutive newlines.
		{
			Name:  "consecutive_newlines",
			Stdin: []byte("a\n\n\nb\n"),
		},

		// Edge: binary data (null bytes).
		{
			Name:  "binary_data",
			Stdin: []byte("line1\x00embedded\nline2\n"),
		},

		// Edge: -s with separator not present in input.
		{
			Name:  "separator_not_in_input",
			Args:  []string{"-s", "|"},
			Stdin: []byte("no separator here\n"),
		},

		// Edge: -s with newline-containing separator.
		{
			Name:  "newline_in_separator",
			Args:  []string{"-s", "\n\n"},
			Stdin: []byte("block1\n\nblock2\n\nblock3\n\n"),
		},

		// Edge: -r with dot-star regex (greedy match).
		{
			Name:  "regex_newline_separator",
			Args:  []string{"-r", "-s", "\n"},
			Stdin: []byte("a\nb\nc\n"),
		},

		// Edge: custom separator, no trailing separator.
		{
			Name:  "custom_sep_no_trailing",
			Args:  []string{"-s", ":"},
			Stdin: []byte("a:b:c"),
		},

		// Edge: -b -s with separator at end (not start).
		{
			Name:  "before_sep_at_end",
			Args:  []string{"-b", "-s", ":"},
			Stdin: []byte("a:b:c:"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
