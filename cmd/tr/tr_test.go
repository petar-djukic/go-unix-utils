// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd054-tr R1.1–R1.4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtr")
	if err != nil {
		t.Skipf("reference binary gtr not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		// R1.1: basic character translation
		{
			Name:  "R1.1_basic_translation",
			Args:  []string{"abc", "xyz"},
			Stdin: []byte("aabbccdd\n"),
		},
		// R1.1: SET2 shorter than SET1, last char pads
		{
			Name:  "R1.1_set2_shorter_pads",
			Args:  []string{"abcd", "xy"},
			Stdin: []byte("abcd\n"),
		},
		// R1.2: empty stdin produces no output
		{
			Name:  "R1.2_empty_stdin",
			Args:  []string{"a", "b"},
			Stdin: []byte{},
		},
		// R1.2: unmatched chars pass through unchanged
		{
			Name:  "R1.2_passthrough_unmatched",
			Args:  []string{"xyz", "abc"},
			Stdin: []byte("hello world\n"),
		},
		// R1.3: character range a-z to A-Z
		{
			Name:  "R1.3_range_lower_to_upper",
			Args:  []string{"a-z", "A-Z"},
			Stdin: []byte("hello world 123\n"),
		},
		// R1.3: digit range translation
		{
			Name:  "R1.3_range_digits",
			Args:  []string{"0-9", "a-j"},
			Stdin: []byte("test 0123456789 end\n"),
		},
		// R1.3: octal escape sequence
		{
			Name:  "R1.3_octal_escape",
			Args:  []string{"\\141\\142", "XY"},
			Stdin: []byte("abc\n"),
		},
		// R1.3: backslash tab escape
		{
			Name:  "R1.3_backslash_tab",
			Args:  []string{"\\t", " "},
			Stdin: []byte("a\tb\tc\n"),
		},
		// R1.3: backslash newline escape
		{
			Name:  "R1.3_backslash_newline",
			Args:  []string{"\\n", "X"},
			Stdin: []byte("a\nb\n"),
		},
		// R1.3: explicit repetition count [c*N]
		{
			Name:  "R1.3_repetition_count",
			Args:  []string{"abc", "[x*3]"},
			Stdin: []byte("abc def\n"),
		},
		// R1.3: fill repetition [c*]
		{
			Name:  "R1.3_repetition_fill",
			Args:  []string{"abcde", "[x*]"},
			Stdin: []byte("abcde fgh\n"),
		},
		// R1.4: POSIX class [:lower:] to [:upper:]
		{
			Name:  "R1.4_class_lower_to_upper",
			Args:  []string{"[:lower:]", "[:upper:]"},
			Stdin: []byte("hello world\n"),
		},
		// R1.4: POSIX class [:upper:] to [:lower:]
		{
			Name:  "R1.4_class_upper_to_lower",
			Args:  []string{"[:upper:]", "[:lower:]"},
			Stdin: []byte("HELLO WORLD\n"),
		},
		// R1.4: POSIX [:digit:] class translation
		{
			Name:  "R1.4_class_digit",
			Args:  []string{"[:digit:]", "abcdefghij"},
			Stdin: []byte("test 0123456789 end\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
