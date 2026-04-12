// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// clearOutput normalizes output to empty for error-path tests where
// only the exit code matters (R5.1). Stderr messages differ between
// the Go binary and GNU reference in format and program name.
func clearOutput(b []byte) []byte { return nil }

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gptx")
	if err != nil {
		t.Skipf("reference binary gptx not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		// --- R1.1: basic KWIC index generation ---
		{
			Name:  "single_line",
			Stdin: []byte("the quick brown fox\n"),
		},
		{
			Name:  "multiple_lines",
			Stdin: []byte("hello world\nfoo bar baz\n"),
		},
		{
			Name:  "single_word_line",
			Stdin: []byte("hello\n"),
		},

		// --- R2.1: width and gap-size flags ---
		{
			Name:  "width_flag",
			Args:  []string{"-w", "40"},
			Stdin: []byte("one two three four\n"),
		},
		{
			Name:  "width_long_flag",
			Args:  []string{"--width=80"},
			Stdin: []byte("alpha beta gamma delta epsilon\n"),
		},
		{
			Name:  "gap_size_flag",
			Args:  []string{"-g", "5"},
			Stdin: []byte("one two three\n"),
		},
		{
			Name:  "gap_size_long_flag",
			Args:  []string{"--gap-size=1"},
			Stdin: []byte("one two three\n"),
		},
		{
			Name:  "width_and_gap_combined",
			Args:  []string{"-w", "50", "-g", "5"},
			Stdin: []byte("alpha beta gamma delta\n"),
		},

		// --- R2.2: ignore-case flag ---
		{
			Name:  "ignore_case_flag",
			Args:  []string{"-f"},
			Stdin: []byte("Alpha beta Gamma\n"),
		},
		{
			Name:  "ignore_case_long_flag",
			Args:  []string{"--ignore-case"},
			Stdin: []byte("Alpha BETA gamma\n"),
		},

		// --- R3.1: word regexp ---
		{
			Name:  "word_regexp_short",
			Args:  []string{"-W", "[a-zA-Z]+"},
			Stdin: []byte("hello-world foo_bar\n"),
		},
		{
			Name:  "word_regexp_long",
			Args:  []string{"--word-regexp=[A-Z][a-z]*"},
			Stdin: []byte("Hello World foo bar\n"),
		},
		{
			Name:  "word_regexp_digits_only",
			Args:  []string{"-W", "[0-9]+"},
			Stdin: []byte("abc 123 def 456\n"),
		},

		// --- R4.1: auto-reference ---
		{
			Name:  "auto_reference_stdin",
			Args:  []string{"-A"},
			Stdin: []byte("hello world\nfoo bar\n"),
		},
		{
			Name:  "auto_reference_right",
			Args:  []string{"-A", "-R"},
			Stdin: []byte("hello world\nfoo bar\n"),
		},
		{
			Name:  "auto_ref_single_line",
			Args:  []string{"-A"},
			Stdin: []byte("alpha beta gamma\n"),
		},
		{
			Name:  "right_ref_width",
			Args:  []string{"-A", "-R", "-w", "60"},
			Stdin: []byte("one two three\n"),
		},

		// --- R4.2: references (-r) ---
		{
			Name:  "sentence_refs",
			Args:  []string{"-r"},
			Stdin: []byte("p1 hello world\np2 foo bar\n"),
		},
		{
			Name:  "sentence_refs_right",
			Args:  []string{"-r", "-R"},
			Stdin: []byte("p1 hello world\np2 foo bar\n"),
		},

		// --- R5.1: exit 0 on success, exit 1 on error ---
		{
			Name:      "error_nonexistent_file",
			Args:      []string{"/nonexistent/path/no_such_file.txt"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			Name:      "error_invalid_long_flag",
			Args:      []string{"--no-such-option"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},

		// --- R5.2: edge cases and boundary conditions ---
		{
			Name:  "empty_input",
			Stdin: []byte(""),
		},
		{
			Name:  "only_newline",
			Stdin: []byte("\n"),
		},
		{
			Name:  "multiple_blank_lines",
			Stdin: []byte("\n\n\n"),
		},
		{
			Name:  "whitespace_only_line",
			Stdin: []byte("   \t  \n"),
		},
		{
			Name:  "single_character_words",
			Stdin: []byte("a b c d e\n"),
		},
		{
			Name:  "repeated_word",
			Stdin: []byte("foo foo foo\n"),
		},
		{
			Name:  "many_short_lines",
			Stdin: []byte("a\nb\nc\nd\ne\nf\ng\n"),
		},
		{
			Name:  "special_chars_in_text",
			Stdin: []byte("hello! world? foo, bar.\n"),
		},
		{
			Name:  "mixed_case_sorting",
			Args:  []string{"-f"},
			Stdin: []byte("Zebra apple BANANA cherry\n"),
		},
		{
			Name:  "very_wide_width",
			Args:  []string{"-w", "200"},
			Stdin: []byte("alpha beta gamma\n"),
		},
		{
			Name:  "large_gap",
			Args:  []string{"-g", "10"},
			Stdin: []byte("one two three\n"),
		},
		{
			Name:  "ignore_case_with_auto_ref",
			Args:  []string{"-f", "-A"},
			Stdin: []byte("Apple BANANA cherry\n"),
		},
		{
			Name:  "refs_single_word_line",
			Args:  []string{"-r"},
			Stdin: []byte("onlyref\n"),
		},
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("hello world\n"),
		},
		{
			Name:  "width_with_refs_right",
			Args:  []string{"-r", "-R", "-w", "50"},
			Stdin: []byte("ref1 alpha beta\nref2 gamma delta\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
