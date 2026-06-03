// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var binaryNameRe = regexp.MustCompile(`(/\S+/)?g?tr\b`)

func normalizeBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("tr"))
}

var usageHintRe = regexp.MustCompile(`(?m)^(Two strings must be given when translating\.|Only one string may be given when deleting without squeezing repeats\.|Try '.*' for more information\.)\n`)

func normalizeUsageHints(b []byte) []byte {
	return bytes.TrimRight(usageHintRe.ReplaceAll(b, nil), "\n")
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtr")
	if err != nil {
		t.Skip("reference binary gtr not found")
	}
	errNorm := []testutils.NormalizeFunc{normalizeBinaryName, normalizeUsageHints}
	tests := []testutils.DiffTest{
		{Name: "basic_translate", Args: []string{"abc", "xyz"}, Stdin: []byte("abcabc\n")},
		{Name: "case_upper", Args: []string{"a-z", "A-Z"}, Stdin: []byte("hello world\n")},
		{Name: "case_lower", Args: []string{"A-Z", "a-z"}, Stdin: []byte("HELLO WORLD\n")},
		{Name: "single_char", Args: []string{"a", "b"}, Stdin: []byte("banana\n")},
		{Name: "set2_shorter", Args: []string{"abcdef", "xy"}, Stdin: []byte("abcdef\n")},
		{Name: "set2_last_extended", Args: []string{"abc", "x"}, Stdin: []byte("abcabc\n")},
		{Name: "digits_to_hash", Args: []string{"0-9", "##########"}, Stdin: []byte("phone: 555-1234\n")},
		{Name: "range_partial", Args: []string{"a-f", "A-F"}, Stdin: []byte("abcdefgh\n")},
		{Name: "class_lower_upper", Args: []string{"[:lower:]", "[:upper:]"}, Stdin: []byte("hello world\n")},
		{Name: "class_upper_lower", Args: []string{"[:upper:]", "[:lower:]"}, Stdin: []byte("HELLO WORLD\n")},
		{Name: "class_digit_replace", Args: []string{"[:digit:]", "0000000000"}, Stdin: []byte("abc123def456\n")},
		{Name: "octal_newline", Args: []string{"\\012", "X"}, Stdin: []byte("hello\nworld\n")},
		{Name: "octal_tab", Args: []string{"\\011", " "}, Stdin: []byte("col1\tcol2\n")},
		{Name: "escape_tab", Args: []string{"\\t", " "}, Stdin: []byte("col1\tcol2\n")},
		{Name: "escape_newline", Args: []string{"\\n", "X"}, Stdin: []byte("hello\nworld\n")},
		{Name: "escape_backslash", Args: []string{"\\\\", "X"}, Stdin: []byte("a\\b\\c\n")},
		{Name: "no_match", Args: []string{"xyz", "XYZ"}, Stdin: []byte("hello\n")},
		{Name: "empty_input", Args: []string{"abc", "xyz"}, Stdin: []byte{}},
		{Name: "binary_data", Args: []string{"\\000", "X"}, Stdin: []byte{0, 'a', 0, 'b', 0, '\n'}},
		{Name: "repeat_set2", Args: []string{"abcde", "[x*]"}, Stdin: []byte("abcde\n")},
		{Name: "repeat_count", Args: []string{"abcde", "[x*5]"}, Stdin: []byte("abcde\n")},
		{Name: "mixed_range_literal", Args: []string{"a-zA-Z", "A-Za-z"}, Stdin: []byte("Hello World\n")},
		{Name: "delete_char", Args: []string{"-d", "l"}, Stdin: []byte("hello\n")},
		{Name: "delete_range", Args: []string{"-d", "a-c"}, Stdin: []byte("abcdefg\n")},
		{Name: "delete_class_digit", Args: []string{"-d", "[:digit:]"}, Stdin: []byte("hello 123\n")},
		{Name: "delete_long", Args: []string{"--delete", "l"}, Stdin: []byte("hello\n")},
		{Name: "squeeze_single_set", Args: []string{"-s", "a-c"}, Stdin: []byte("aabbcc\n")},
		{Name: "squeeze_spaces", Args: []string{"-s", " "}, Stdin: []byte("hello   world\n")},
		{Name: "squeeze_with_translate", Args: []string{"-s", "a-z", "A-Z"}, Stdin: []byte("aabbcc\n")},
		{Name: "squeeze_long", Args: []string{"--squeeze-repeats", "o"}, Stdin: []byte("fooood\n")},
		{Name: "delete_squeeze_ds", Args: []string{"-ds", "[:digit:]", " "}, Stdin: []byte("1 2  3  hello\n")},
		{Name: "delete_squeeze_separate", Args: []string{"-d", "-s", "0-9", "a-z"}, Stdin: []byte("aa11bb22cc\n")},
		{Name: "complement_translate", Args: []string{"-c", "a-z\\n", "*"}, Stdin: []byte("hello world\n")},
		{Name: "complement_delete", Args: []string{"-cd", "a-z\\n"}, Stdin: []byte("hello 123 world\n")},
		{Name: "complement_squeeze", Args: []string{"-cs", "a-z", "\\n"}, Stdin: []byte("hello 123 world\n")},
		{Name: "complement_upper_C", Args: []string{"-Cd", "a-z\\n"}, Stdin: []byte("hello 123\n")},
		{Name: "complement_long", Args: []string{"--complement", "-d", "a-z\\n"}, Stdin: []byte("hello 123\n")},

		// R3.1: Character class translation pairs
		{Name: "r3_lower_to_upper_mixed", Args: []string{"[:lower:]", "[:upper:]"}, Stdin: []byte("Hello World 123\n")},
		{Name: "r3_upper_to_lower_mixed", Args: []string{"[:upper:]", "[:lower:]"}, Stdin: []byte("Hello World 123\n")},
		{Name: "r3_lower_upper_all_alpha", Args: []string{"[:lower:]", "[:upper:]"}, Stdin: []byte("abcdefghijklmnopqrstuvwxyz\n")},
		{Name: "r3_upper_lower_all_alpha", Args: []string{"[:upper:]", "[:lower:]"}, Stdin: []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ\n")},
		{Name: "r3_lower_upper_empty", Args: []string{"[:lower:]", "[:upper:]"}, Stdin: []byte("")},
		{Name: "r3_lower_upper_no_alpha", Args: []string{"[:lower:]", "[:upper:]"}, Stdin: []byte("12345!@#$%\n")},

		// R3.2: SET2 must not be empty when translating
		{Name: "r3_empty_set2", Args: []string{"a-z", ""}, Stdin: []byte("hello\n"), ExitCode: 1, Normalize: errNorm},
		{Name: "r3_missing_operand", Args: []string{}, Stdin: []byte("hello\n"), ExitCode: 1, Normalize: errNorm},
		{Name: "r3_missing_set2", Args: []string{"abc"}, Stdin: []byte("hello\n"), ExitCode: 1, Normalize: errNorm},

		// R3.3: Equivalence classes [=CHAR=]
		{Name: "r3_equiv_set1", Args: []string{"[=a=]", "b"}, Stdin: []byte("abcabc\n")},
		{Name: "r3_equiv_delete", Args: []string{"-d", "[=l=]"}, Stdin: []byte("hello\n")},
		{Name: "r3_equiv_squeeze", Args: []string{"-s", "[=l=]"}, Stdin: []byte("hello\n")},
		{Name: "r3_equiv_set2_translate_err", Args: []string{"a", "[=b=]"}, Stdin: []byte("aaa\n"), ExitCode: 1, Normalize: errNorm},
		{Name: "r3_equiv_set2_ds_ok", Args: []string{"-ds", "[:digit:]", "[=a=]"}, Stdin: []byte("aa11bb\n")},

		// R4.1: Exit 0 on successful translation/deletion
		{Name: "r4_exit0_translate", Args: []string{"a", "b"}, Stdin: []byte("a\n"), ExitCode: 0},
		{Name: "r4_exit0_delete", Args: []string{"-d", "a"}, Stdin: []byte("abc\n"), ExitCode: 0},
		{Name: "r4_exit0_squeeze", Args: []string{"-s", "a"}, Stdin: []byte("aaa\n"), ExitCode: 0},
		{Name: "r4_exit0_complement", Args: []string{"-cd", "a-z\\n"}, Stdin: []byte("hello 123\n"), ExitCode: 0},
		{Name: "r4_exit0_ds_combined", Args: []string{"-ds", "[:digit:]", " "}, Stdin: []byte("a1 b2\n"), ExitCode: 0},

		// R4.2: Exit 1 on usage errors
		{Name: "r4_exit1_invalid_class", Args: []string{"[:bogus:]", "x"}, Stdin: []byte("a\n"), ExitCode: 1, Normalize: errNorm},
		{Name: "r4_exit1_invalid_option", Args: []string{"-z", "a", "b"}, Stdin: []byte("a\n"), ExitCode: 1, Normalize: errNorm},
		{Name: "r4_exit1_no_args", Args: []string{}, Stdin: []byte("a\n"), ExitCode: 1, Normalize: errNorm},
				{Name: "r4_exit1_delete_extra", Args: []string{"-d", "a", "b"}, Stdin: []byte("a\n"), ExitCode: 1, Normalize: errNorm},
		{Name: "r4_exit1_reverse_range", Args: []string{"z-a", "A-Z"}, Stdin: []byte("a\n"), ExitCode: 1, Normalize: errNorm},

		// R4.3: Comprehensive differential coverage
		{Name: "r4_diff_basic", Args: []string{"aeiou", "AEIOU"}, Stdin: []byte("the quick brown fox\n")},
		{Name: "r4_diff_delete_class", Args: []string{"-d", "[:space:]"}, Stdin: []byte("hello world\n")},
		{Name: "r4_diff_squeeze_class", Args: []string{"-s", "[:space:]"}, Stdin: []byte("hello   world\n")},
		{Name: "r4_diff_complement_class", Args: []string{"-cd", "[:alpha:]\\n"}, Stdin: []byte("abc 123 def\n")},
		{Name: "r4_diff_ds_class", Args: []string{"-ds", "[:upper:]", "[:lower:]"}, Stdin: []byte("HeLLo WoRLD\n")},
		{Name: "r4_diff_range_octal", Args: []string{"\\141-\\172", "A-Z"}, Stdin: []byte("hello\n")},
		{Name: "r4_diff_case_class", Args: []string{"[:lower:]", "[:upper:]"}, Stdin: []byte("mixed Case 123\n")},
		{Name: "r4_diff_escape_all", Args: []string{"\\a\\b\\f\\n\\r\\t\\v", "ABCDEFG"}, Stdin: []byte{'\a', '\b', '\f', '\n', '\r', '\t', '\v'}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
