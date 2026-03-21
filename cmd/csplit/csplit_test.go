// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements differential tests for prd068-csplit R1.1–R1.4, R2.1–R2.4.
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
			// R2.2 + R2.4: {*} with first match failing is an error.
			Name:      "star_no_first_match",
			Args:      []string{"-", "/zzz/", "{*}"},
			Stdin:     []byte("a\nb\nc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normProgName},
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
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
