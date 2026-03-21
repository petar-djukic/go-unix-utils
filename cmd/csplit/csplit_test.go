// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements differential tests for prd068-csplit R1.1–R1.4, R2.1–R2.4,
// R3.1–R3.4 (error handling and edge cases).
package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normProgName normalizes the program name in stderr output. The reference
// binary may use a full path like /opt/homebrew/bin/csplit as its argv[0].
func normProgName(b []byte) []byte {
	lines := strings.Split(string(b), "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "csplit:"); idx > 0 {
			lines[i] = line[idx:]
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// normRegexErr normalizes regex error messages. Go and GNU regex libraries
// produce different error detail strings, so we replace the entire line
// containing "invalid regular expression" with a fixed string.
func normRegexErr(b []byte) []byte {
	lines := strings.Split(string(b), "\n")
	for i, line := range lines {
		if strings.Contains(line, "invalid regular expression") {
			lines[i] = "invalid regular expression"
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcsplit")
	if err != nil {
		t.Skipf("reference binary gcsplit not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		// === R1 tests ===
		{
			// R1.2: /REGEXP/ splits at the next line matching REGEXP.
			Name:  "regex_split",
			Args:  []string{"-", "/c/"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
		{
			// R1.4: INTEGER splits at the given line number.
			Name:  "line_number_split",
			Args:  []string{"-", "4", "7"},
			Stdin: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"),
		},
		{
			// R1.3: %REGEXP% skips to the matching line without output.
			Name:  "skip_pattern",
			Args:  []string{"-", "%c%"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
		{
			// R1.1: multiple patterns applied in order.
			Name:  "multiple_regex_patterns",
			Args:  []string{"-", "/b/", "/d/"},
			Stdin: []byte("a\nb\nc\nd\ne\n"),
		},
		{
			// R1.1 + R1.2 + R1.4: mixed regex and line number patterns.
			Name:  "mixed_patterns",
			Args:  []string{"-", "/c/", "5"},
			Stdin: []byte("a\nb\nc\nd\ne\nf\n"),
		},
		{
			// R1.3 + R1.2: skip pattern followed by regex split.
			Name:  "skip_then_regex",
			Args:  []string{"-", "%b%", "/d/"},
			Stdin: []byte("a\nb\nc\nd\ne\n"),
		},
		// === R2 tests ===
		{
			// R2.1: {N} repeats the pattern N additional times.
			Name:  "repeat_count_fixed",
			Args:  []string{"-", "/[adf]/", "{2}"},
			Stdin: []byte("a\nb\nc\nd\ne\nf\ng\n"),
		},
		{
			// R2.2: {*} repeats as many times as possible.
			Name:  "repeat_star",
			Args:  []string{"-", "/x/", "{*}"},
			Stdin: []byte("1\nx\n2\nx\n3\nx\n4\n"),
		},
		{
			// R2.3: /REGEXP/+N splits N lines after the match.
			Name:  "offset_positive",
			Args:  []string{"-", "/3/+1"},
			Stdin: []byte("1\n2\n3\n4\n5\n"),
		},
		{
			// R2.3: /REGEXP/-N splits N lines before the match.
			Name:  "offset_negative",
			Args:  []string{"-", "/3/-1"},
			Stdin: []byte("1\n2\n3\n4\n5\n"),
		},
		{
			// R2.4: no match produces error and exit 1.
			Name:      "no_match_error",
			Args:      []string{"-", "/zzz/"},
			Stdin:     []byte("a\nb\nc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normProgName},
		},
		{
			// R2.2: {*} with no matches is not an error; outputs all input.
			Name:  "star_no_first_match",
			Args:  []string{"-", "/zzz/", "{*}"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R2.1: {0} is equivalent to a single application.
			Name:  "repeat_zero",
			Args:  []string{"-", "/c/", "{0}"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
		{
			// R2.2 + R2.3: offset combined with star repeat.
			Name:  "offset_with_star",
			Args:  []string{"-", "/x/+1", "{*}"},
			Stdin: []byte("1\nx\n2\nx\n3\n"),
		},
		// === R3 error handling and edge case tests ===
		{
			// R3.1: invalid regex pattern exits 1.
			Name:      "invalid_regex",
			Args:      []string{"-", "/[/"},
			Stdin:     []byte("a\nb\nc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normProgName, normRegexErr},
		},
		{
			// R3.2: regex pattern on empty input exits 1 (no match).
			Name:      "empty_input_regex",
			Args:      []string{"-", "/a/"},
			Stdin:     []byte{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normProgName},
		},
		{
			// R3.2: line number on empty input exits 1 (out of range).
			Name:      "empty_input_line_num",
			Args:      []string{"-", "3"},
			Stdin:     []byte{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normProgName},
		},
		{
			// R3.1: line number 0 is invalid, exits 1.
			Name:      "line_number_zero",
			Args:      []string{"-", "0"},
			Stdin:     []byte("a\nb\nc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normProgName},
		},
		{
			// R3.2: line number beyond EOF exits 1.
			Name:      "line_number_beyond_eof",
			Args:      []string{"-", "10"},
			Stdin:     []byte("a\nb\nc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normProgName},
		},
		{
			// R3.3: regex matching first line creates zero-length first piece.
			Name:  "zero_length_first_piece",
			Args:  []string{"-", "/a/"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R3.3: -z elides the empty first piece when regex matches line 1.
			Name:  "elide_empty_first_piece",
			Args:  []string{"-z", "-", "/a/"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R3.2: repeat count exceeds available matches (premature EOF).
			Name:      "repeat_premature_eof",
			Args:      []string{"-", "/x/", "{3}"},
			Stdin:     []byte("1\nx\n2\nx\n3\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normProgName},
		},
		{
			// R3.4: successful split exits 0.
			Name:  "success_exit_zero",
			Args:  []string{"-", "3"},
			Stdin: []byte("a\nb\nc\nd\ne\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
