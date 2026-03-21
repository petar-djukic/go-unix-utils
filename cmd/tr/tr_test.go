// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd054-tr R1.1–R1.4, R2.1–R2.4.
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
		// R2.1: delete single character
		{
			Name:  "R2.1_delete_char",
			Args:  []string{"-d", "l"},
			Stdin: []byte("hello\n"),
		},
		// R2.1: delete with POSIX class
		{
			Name:  "R2.1_delete_digits",
			Args:  []string{"-d", "[:digit:]"},
			Stdin: []byte("hello 123\n"),
		},
		// R2.1: delete with range
		{
			Name:  "R2.1_delete_range",
			Args:  []string{"-d", "a-c"},
			Stdin: []byte("abcdef\n"),
		},
		// R2.1: delete with long flag
		{
			Name:  "R2.1_delete_long_flag",
			Args:  []string{"--delete", "l"},
			Stdin: []byte("hello\n"),
		},
		// R2.2: squeeze single character
		{
			Name:  "R2.2_squeeze_char",
			Args:  []string{"-s", "o"},
			Stdin: []byte("foooobar\n"),
		},
		// R2.2: squeeze range
		{
			Name:  "R2.2_squeeze_range",
			Args:  []string{"-s", "a-c"},
			Stdin: []byte("aabbcc\n"),
		},
		// R2.2: squeeze spaces
		{
			Name:  "R2.2_squeeze_spaces",
			Args:  []string{"-s", " "},
			Stdin: []byte("hello   world\n"),
		},
		// R2.2: translate and squeeze
		{
			Name:  "R2.2_translate_squeeze",
			Args:  []string{"-s", "abc", "xyz"},
			Stdin: []byte("aabbcc\n"),
		},
		// R2.3: delete and squeeze combined
		{
			Name:  "R2.3_delete_squeeze",
			Args:  []string{"-ds", "aeiou", "a-z"},
			Stdin: []byte("aabbccdd\n"),
		},
		// R2.3: delete and squeeze with classes
		{
			Name:  "R2.3_delete_squeeze_class",
			Args:  []string{"-ds", "[:upper:]", "[:lower:]"},
			Stdin: []byte("AABBccddEE\n"),
		},
		// R2.4: complement delete
		{
			Name:  "R2.4_complement_delete",
			Args:  []string{"-cd", "a-z\\n"},
			Stdin: []byte("hello 123 world\n"),
		},
		// R2.4: complement translate
		{
			Name:  "R2.4_complement_translate",
			Args:  []string{"-c", "a-z\\n", "*"},
			Stdin: []byte("hello world 123\n"),
		},
		// R2.4: complement squeeze
		{
			Name:  "R2.4_complement_squeeze",
			Args:  []string{"-cs", "a-z\\n", "*"},
			Stdin: []byte("hello  123  world\n"),
		},
		// R2.4: complement with -C flag variant
		{
			Name:  "R2.4_complement_C_flag",
			Args:  []string{"-Cd", "[:alpha:]\\n"},
			Stdin: []byte("abc 123 def\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
