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
	refBin, err := exec.LookPath("gtr")
	if err != nil {
		t.Skip("reference binary gtr not found")
	}
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
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
