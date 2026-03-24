// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tr against GNU gtr.
// Covers prd054-tr R1.1–R1.4, R2.1–R2.4, R3.1–R3.3, R4.1–R4.3.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes error messages between GNU gtr and Go tr.
// GNU tr prints supplementary hint lines after certain errors; strip them.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?tr|gtr`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	hintLines := regexp.MustCompile(
		`(?m)^(Two strings must be given|Only one string may be given)[^\n]*\n?`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("tr"))
		b = tryHelp.ReplaceAll(b, nil)
		b = hintLines.ReplaceAll(b, nil)
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtr")
	if err != nil {
		t.Skipf("reference binary gtr not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		// --- R1: Character translation ---

		// R1.1: basic lowercase to uppercase translation via ranges.
		{
			Name:  "translate_lower_to_upper",
			Args:  []string{"a-z", "A-Z"},
			Stdin: []byte("hello world\n"),
		},
		// R1.1: single character translation.
		{
			Name:  "translate_single_char",
			Args:  []string{"a", "b"},
			Stdin: []byte("abcabc\n"),
		},
		// R1.1: SET2 shorter than SET1 — last char of SET2 pads.
		{
			Name:  "translate_set2_shorter",
			Args:  []string{"abc", "x"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.3: octal escape in SET.
		{
			Name:  "translate_octal_escape",
			Args:  []string{"\\141", "X"},
			Stdin: []byte("abc\n"),
		},
		// R1.3: backslash escapes (\\n, \\t).
		{
			Name:  "translate_backslash_n",
			Args:  []string{"\\n", "X"},
			Stdin: []byte("hello\nworld\n"),
		},
		// R1.3: backslash-t escape.
		{
			Name:  "translate_backslash_t",
			Args:  []string{"\\t", " "},
			Stdin: []byte("hello\tworld\n"),
		},
		// R1.3: backslash-backslash escape.
		{
			Name:  "translate_backslash_backslash",
			Args:  []string{"\\\\", "X"},
			Stdin: []byte("a\\b\\c\n"),
		},
		// R1.3: range a-z to A-Z (explicit range syntax).
		{
			Name:  "translate_range_az_AZ",
			Args:  []string{"a-z", "A-Z"},
			Stdin: []byte("the quick brown fox\n"),
		},

		// --- R1.4 / R3.1: POSIX character classes ---

		// R1.4: [:lower:] to [:upper:] case conversion.
		{
			Name:  "class_lower_to_upper",
			Args:  []string{"[:lower:]", "[:upper:]"},
			Stdin: []byte("hello world\n"),
		},
		// R1.4: [:upper:] to [:lower:] case conversion.
		{
			Name:  "class_upper_to_lower",
			Args:  []string{"[:upper:]", "[:lower:]"},
			Stdin: []byte("HELLO WORLD\n"),
		},
		// R1.4: [:digit:] delete via -d.
		{
			Name:  "class_delete_digits",
			Args:  []string{"-d", "[:digit:]"},
			Stdin: []byte("abc123def456\n"),
		},
		// R1.4: [:alpha:] delete.
		{
			Name:  "class_delete_alpha",
			Args:  []string{"-d", "[:alpha:]"},
			Stdin: []byte("abc123def456\n"),
		},
		// R1.4: [:space:] squeeze.
		{
			Name:  "class_squeeze_space",
			Args:  []string{"-s", "[:space:]"},
			Stdin: []byte("hello   world\t\tfoo\n"),
		},
		// R1.4: [:alnum:] complement delete.
		{
			Name:  "class_complement_delete_alnum",
			Args:  []string{"-cd", "[:alnum:]\\n"},
			Stdin: []byte("hello, world! 123.\n"),
		},
		// R1.4: [:punct:] delete.
		{
			Name:  "class_delete_punct",
			Args:  []string{"-d", "[:punct:]"},
			Stdin: []byte("hello, world! foo-bar.\n"),
		},

		// --- R2: Delete and squeeze modes ---

		// R2.1: -d deletes characters in SET1.
		{
			Name:  "delete_single_char",
			Args:  []string{"-d", "l"},
			Stdin: []byte("hello\n"),
		},
		// R2.1: -d with range.
		{
			Name:  "delete_range",
			Args:  []string{"-d", "a-f"},
			Stdin: []byte("abcdefghijklm\n"),
		},
		// R2.2: -s squeezes repeated characters.
		{
			Name:  "squeeze_repeated",
			Args:  []string{"-s", "a-c"},
			Stdin: []byte("aabbcc\n"),
		},
		// R2.2: -s with translate — squeeze applies to SET2 after translation.
		{
			Name:  "translate_then_squeeze",
			Args:  []string{"-s", "abc", "x"},
			Stdin: []byte("aabbcc\n"),
		},
		// R2.3: -ds combined mode — delete SET1, squeeze SET2.
		{
			Name:  "delete_and_squeeze",
			Args:  []string{"-ds", "[:digit:]", "[:space:]"},
			Stdin: []byte("a 1  b 2  c 3\n"),
		},
		// R2.4: -c complement with translation.
		{
			Name:  "complement_translate",
			Args:  []string{"-c", "a-z\\n", "*"},
			Stdin: []byte("hello 123 world\n"),
		},
		// R2.4: -c complement with delete.
		{
			Name:  "complement_delete",
			Args:  []string{"-cd", "a-z\\n"},
			Stdin: []byte("hello 123 world\n"),
		},
		// R2.4: -C as synonym for -c.
		{
			Name:  "complement_C_flag",
			Args:  []string{"-Cd", "a-z\\n"},
			Stdin: []byte("hello 123 world\n"),
		},
		// R2.4: -cs complement squeeze.
		{
			Name:  "complement_squeeze",
			Args:  []string{"-cs", "a-z", "\\n"},
			Stdin: []byte("hello 123 world\n"),
		},

		// --- R3: Character class translation pairs ---

		// R3.2: translate mode with empty SET2 — error.
		{
			Name:      "error_translate_no_set2",
			Args:      []string{"abc"},
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{errNorm},
		},

		// --- R4: Exit codes and error cases ---

		// R4.1: successful translation exits 0.
		{
			Name:  "exit_zero_on_success",
			Args:  []string{"a", "b"},
			Stdin: []byte("a\n"),
		},
		// R4.2: missing operand — exit 1.
		{
			Name:      "error_missing_operand",
			Args:      []string{},
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.2: extra operand with -d (no -s).
		{
			Name:      "error_extra_operand_delete",
			Args:      []string{"-d", "abc", "xyz"},
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.2: too many operands.
		{
			Name:      "error_too_many_operands",
			Args:      []string{"a", "b", "c"},
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.2: invalid flag.
		{
			Name:      "error_invalid_flag",
			Args:      []string{"-x", "abc"},
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{errNorm},
		},

		// --- R6: Edge cases ---

		// Empty stdin produces empty output.
		{
			Name:  "empty_stdin",
			Args:  []string{"a", "b"},
			Stdin: []byte{},
		},
		// Binary data passes through with translation.
		{
			Name:  "binary_data",
			Args:  []string{"\\000", "X"},
			Stdin: []byte{0x00, 0x01, 0x02, 0x00, '\n'},
		},
		// Single-character SET translation.
		{
			Name:  "single_char_sets",
			Args:  []string{"x", "y"},
			Stdin: []byte("xylophone\n"),
		},
		// Delete with empty result.
		{
			Name:  "delete_all_chars",
			Args:  []string{"-d", "a-z"},
			Stdin: []byte("abcdef\n"),
		},
		// Squeeze with no repeats — input unchanged.
		{
			Name:  "squeeze_no_repeats",
			Args:  []string{"-s", "abc"},
			Stdin: []byte("abcabc\n"),
		},
		// Squeeze only specific characters, leave others.
		{
			Name:  "squeeze_selective",
			Args:  []string{"-s", "o"},
			Stdin: []byte("foooobar\n"),
		},
		// Multiple flags combined: -cs with ranges.
		{
			Name:  "complement_squeeze_ranges",
			Args:  []string{"-cs", "[:alpha:]", "\\n"},
			Stdin: []byte("hello 123 world\n"),
		},
		// Translation with identical SET1 and SET2 (identity).
		{
			Name:  "identity_translation",
			Args:  []string{"abc", "abc"},
			Stdin: []byte("abcdef\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
