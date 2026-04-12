// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main tests cmd/csplit via differential testing against gcsplit.
// Implements srd068-csplit R4.3, R4.4.
package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeProgName strips the binary name prefix from stderr lines so that
// error messages from "csplit" and "gcsplit" compare equal.
var normalizeProgName = func(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		if _, after, found := bytes.Cut(line, []byte(": ")); found {
			lines[i] = after
		}
	}
	return bytes.Join(lines, []byte("\n"))
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcsplit")
	if err != nil {
		t.Skipf("reference binary gcsplit not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		{
			// R1.2: /REGEXP/ splits at matching line.
			Name:  "R1.2_regex_split",
			Args:  []string{"-", "/c/"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
		{
			// R1.3: %REGEXP% skips to match without output file.
			Name:  "R1.3_skip_pattern",
			Args:  []string{"-", "%c%"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
		{
			// R1.4: INTEGER splits at the given line number.
			Name:  "R1.4_line_number",
			Args:  []string{"-", "3"},
			Stdin: []byte("1\n2\n3\n4\n5\n"),
		},
		{
			// R1.1: multiple patterns applied in order (regex).
			Name:  "R1.1_multiple_regex_patterns",
			Args:  []string{"-", "/c/", "/e/"},
			Stdin: []byte("a\nb\nc\nd\ne\nf\n"),
		},
		{
			// R1.1 + R1.4: multiple INTEGER patterns.
			Name:  "R1.4_multiple_line_numbers",
			Args:  []string{"-", "4", "7"},
			Stdin: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"),
		},
		{
			// R1.1: mixed pattern types applied in order.
			Name:  "R1.1_mixed_line_and_regex",
			Args:  []string{"-", "3", "/e/"},
			Stdin: []byte("a\nb\nc\nd\ne\nf\n"),
		},
		{
			// R1.3 + R1.2: skip then regex split.
			Name:  "R1.3_skip_then_regex_split",
			Args:  []string{"-", "%c%", "/e/"},
			Stdin: []byte("a\nb\nc\nd\ne\nf\n"),
		},
		{
			// R2.1: {N} repeats the previous regex pattern N additional times.
			Name:  "R2.1_repeat_count_regex",
			Args:  []string{"-", "/a/", "{2}"},
			Stdin: []byte("x\na\ny\na\nz\na\nw\n"),
		},
		{
			// R2.1: {N} repeats a line number pattern as relative offset.
			Name:  "R2.1_repeat_count_line_number",
			Args:  []string{"-", "3", "{2}"},
			Stdin: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n"),
		},
		{
			// R2.2: {*} repeats the regex pattern until end of input.
			Name:  "R2.2_repeat_star_regex",
			Args:  []string{"-", "/a/", "{*}"},
			Stdin: []byte("x\na\ny\na\nz\na\nw\n"),
		},
		{
			// R2.3: /REGEXP/+N splits N lines after the matching line.
			Name:  "R2.3_offset_positive",
			Args:  []string{"-", "/c/+1"},
			Stdin: []byte("a\nb\nc\nd\ne\n"),
		},
		{
			// R2.3: /REGEXP/-N splits N lines before the matching line.
			Name:  "R2.3_offset_negative",
			Args:  []string{"-", "/c/-1"},
			Stdin: []byte("a\nb\nc\nd\ne\n"),
		},
		{
			// R2.4: error when pattern does not match any line.
			Name:      "R2.4_no_match_error",
			Args:      []string{"-", "/nomatch/"},
			Stdin:     []byte("a\nb\nc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgName},
		},
		{
			// R3.1: default output filenames xx00, xx01, etc.
			Name:  "R3.1_default_prefix_and_suffix",
			Args:  []string{"-", "3"},
			Stdin: []byte("1\n2\n3\n4\n5\n"),
		},
		{
			// R3.2: -f PREFIX changes output filename prefix.
			Name:  "R3.2_custom_prefix_short",
			Args:  []string{"-f", "chunk", "-", "/c/"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
		{
			// R3.2: --prefix=PREFIX long form.
			Name:  "R3.2_custom_prefix_long",
			Args:  []string{"--prefix=out", "-", "/c/"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
		{
			// R3.3: -n DIGITS changes suffix digit width.
			Name:  "R3.3_custom_digits_short",
			Args:  []string{"-n", "4", "-", "/c/"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
		{
			// R3.3: --digits=DIGITS long form.
			Name:  "R3.3_custom_digits_long",
			Args:  []string{"--digits=3", "-", "/c/"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
		{
			// R3.2 + R3.3: combined prefix and digit width.
			Name:  "R3.2_R3.3_prefix_and_digits",
			Args:  []string{"-f", "part", "-n", "3", "-", "3", "5"},
			Stdin: []byte("1\n2\n3\n4\n5\n6\n7\n"),
		},
		{
			// R3.4: -z suppresses empty output files.
			// Splitting at the first line creates an empty first piece.
			Name:  "R3.4_elide_empty_short",
			Args:  []string{"-z", "-", "/^a/"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R3.4: --elide-empty-files long form.
			Name:  "R3.4_elide_empty_long",
			Args:  []string{"--elide-empty-files", "-", "/^b/"},
			Stdin: []byte("b\nc\nd\n"),
		},
		{
			// R4.1: exit 0 when input is split successfully.
			Name:  "R4.1_exit_0_success",
			Args:  []string{"-", "3"},
			Stdin: []byte("a\nb\nc\nd\ne\n"),
		},
		{
			// R4.2: exit 1 when a pattern fails to match.
			Name:      "R4.2_exit_1_no_match",
			Args:      []string{"-", "/zzz/"},
			Stdin:     []byte("a\nb\nc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgName},
		},
		{
			// R4.2: exit 1 for invalid option.
			Name:     "R4.2_exit_1_invalid_option",
			Args:     []string{"--bogus-flag", "-", "/a/"},
			Stdin:    []byte("a\nb\n"),
			ExitCode: 1,
			// Stderr format differs between GNU (includes "Try --help") and Go.
			Normalize: []testutils.NormalizeFunc{func(b []byte) []byte { return nil }},
		},
		{
			// R4.4: invalid regex produces an error and exits 1.
			Name:     "R4.4_invalid_regex_error",
			Args:     []string{"-", "/[invalid/"},
			Stdin:    []byte("a\nb\nc\n"),
			ExitCode: 1,
			// Stderr format differs between GNU regex and Go regexp engines.
			Normalize: []testutils.NormalizeFunc{func(b []byte) []byte { return nil }},
		},
		{
			// R4.4: quiet mode suppresses byte count output.
			Name:  "R4.4_quiet_mode",
			Args:  []string{"-s", "-", "/c/"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
