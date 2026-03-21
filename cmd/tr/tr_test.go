// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd054-tr R1.1–R1.4, R2.1–R2.4, R3.1–R3.3, R4.1–R4.3.
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
		// R3.1: squeeze repeats collapses adjacent identical chars
		{
			Name:  "R3.1_squeeze_repeats_single",
			Args:  []string{"-s", "a"},
			Stdin: []byte("aaabbbccc\n"),
		},
		// R3.1: squeeze repeats with multiple chars in set
		{
			Name:  "R3.1_squeeze_repeats_multi",
			Args:  []string{"-s", "abc"},
			Stdin: []byte("aaabbbccc\n"),
		},
		// R3.1: squeeze with translate and class pair
		{
			Name:  "R3.1_squeeze_translate_class",
			Args:  []string{"-s", "[:lower:]", "[:upper:]"},
			Stdin: []byte("aabbccdd\n"),
		},
		// R3.1: squeeze newlines
		{
			Name:  "R3.1_squeeze_newlines",
			Args:  []string{"-s", "\\n"},
			Stdin: []byte("a\n\n\nb\n\nc\n"),
		},
		// R3.2: delete complement keeps only specified chars
		{
			Name:  "R3.2_delete_complement_digits",
			Args:  []string{"-dc", "[:digit:]"},
			Stdin: []byte("abc123def456\n"),
		},
		// R3.2: delete complement keeps alpha and newline
		{
			Name:  "R3.2_delete_complement_alpha",
			Args:  []string{"-dc", "[:alpha:]\\n"},
			Stdin: []byte("hello 123 world\n"),
		},
		// R3.2: delete complement with range
		{
			Name:  "R3.2_delete_complement_range",
			Args:  []string{"-dc", "a-f\\n"},
			Stdin: []byte("abcdefghij\n"),
		},
		// R3.2: delete complement with -C flag
		{
			Name:  "R3.2_delete_complement_C",
			Args:  []string{"-Cd", "0-9"},
			Stdin: []byte("abc123def456\n"),
		},
		// R3.3: squeeze complement collapses non-set chars
		{
			Name:  "R3.3_squeeze_complement_single_set",
			Args:  []string{"-sc", "[:alpha:]"},
			Stdin: []byte("hello   123   world\n"),
		},
		// R3.3: squeeze complement with translate
		{
			Name:  "R3.3_squeeze_complement_translate",
			Args:  []string{"-cs", "[:alpha:]\\n", "_"},
			Stdin: []byte("hello  123  world\n"),
		},
		// R3.3: squeeze complement with range
		{
			Name:  "R3.3_squeeze_complement_range",
			Args:  []string{"-cs", "a-z\\n", "*"},
			Stdin: []byte("abc  123  def\n"),
		},
		// R3.3: squeeze complement with -C flag
		{
			Name:  "R3.3_squeeze_complement_C_translate",
			Args:  []string{"-Cs", "[:digit:]\\n", " "},
			Stdin: []byte("abc123def456ghi\n"),
		},
		// R4.1: octal escape for 'A' (0101) in SET1
		{
			Name:  "R4.1_octal_escape_A",
			Args:  []string{"\\101", "X"},
			Stdin: []byte("ABCABC\n"),
		},
		// R4.1: octal escape in both SET1 and SET2
		{
			Name:  "R4.1_octal_both_sets",
			Args:  []string{"\\141", "\\132"},
			Stdin: []byte("abc\n"),
		},
		// R4.1: octal escape for null byte (\\000)
		{
			Name:  "R4.1_octal_null",
			Args:  []string{"-d", "\\000"},
			Stdin: []byte("a\x00b\x00c\n"),
		},
		// R4.1: octal escape range
		{
			Name:  "R4.1_octal_range",
			Args:  []string{"\\141-\\172", "A-Z"},
			Stdin: []byte("hello world\n"),
		},
		// R4.2: backslash-a (bell)
		{
			Name:  "R4.2_backslash_a",
			Args:  []string{"\\a", "X"},
			Stdin: []byte("a\ab\n"),
		},
		// R4.2: backslash-b (backspace)
		{
			Name:  "R4.2_backslash_b",
			Args:  []string{"\\b", "X"},
			Stdin: []byte("a\bb\n"),
		},
		// R4.2: backslash-f (form feed)
		{
			Name:  "R4.2_backslash_f",
			Args:  []string{"\\f", "X"},
			Stdin: []byte("a\fb\n"),
		},
		// R4.2: backslash-r (carriage return)
		{
			Name:  "R4.2_backslash_r",
			Args:  []string{"\\r", "X"},
			Stdin: []byte("a\rb\n"),
		},
		// R4.2: backslash-v (vertical tab)
		{
			Name:  "R4.2_backslash_v",
			Args:  []string{"\\v", "X"},
			Stdin: []byte("a\vb\n"),
		},
		// R4.2: backslash-backslash (literal backslash)
		{
			Name:  "R4.2_backslash_backslash",
			Args:  []string{"\\\\", "X"},
			Stdin: []byte("a\\b\\c\n"),
		},
		// R4.3: equivalence class [=a=] as identity under LC_ALL=C
		{
			Name:  "R4.3_equiv_class_identity",
			Args:  []string{"[=a=]", "X"},
			Stdin: []byte("abcabc\n"),
		},
		// R4.3: equivalence class [=Z=] in SET1
		{
			Name:  "R4.3_equiv_class_upper",
			Args:  []string{"[=Z=]", "X"},
			Stdin: []byte("AZBZc\n"),
		},
		// R4.3: equivalence class with delete mode
		{
			Name:  "R4.3_equiv_class_delete",
			Args:  []string{"-d", "[=x=]"},
			Stdin: []byte("axbxcx\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
