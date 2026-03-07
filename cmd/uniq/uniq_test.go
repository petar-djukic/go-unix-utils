// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/uniq against the GNU reference binary (guniq).
// Implements prd028-uniq R1-R4 verification.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1, R1.2: Default deduplication — adjacent duplicates suppressed.
		{
			Name:  "default_dedup",
			Args:  []string{},
			Stdin: []byte("a\na\nb\na\n"),
		},
		// R2.4: -c prefixes each line with its run count.
		{
			Name:  "count",
			Args:  []string{"-c"},
			Stdin: []byte("a\na\nb\n"),
		},
		// R2.1: -d outputs only lines from duplicate runs (one copy per run).
		{
			Name:  "duplicates_only",
			Args:  []string{"-d"},
			Stdin: []byte("a\na\nb\n"),
		},
		// R2.2: -D outputs every line of duplicate runs.
		{
			Name:  "all_duplicates",
			Args:  []string{"-D"},
			Stdin: []byte("a\na\nb\n"),
		},
		// R2.3: -u outputs only truly unique lines.
		{
			Name:  "unique_only",
			Args:  []string{"-u"},
			Stdin: []byte("a\na\nb\n"),
		},
		// R3.1: -i treats upper and lower case as equal.
		{
			Name:  "case_insensitive",
			Args:  []string{"-i"},
			Stdin: []byte("A\na\nb\n"),
		},
		// R3.2: -f 1 skips the first field when comparing lines.
		{
			Name:  "skip_fields",
			Args:  []string{"-f", "1"},
			Stdin: []byte("key1 val\nkey2 val\nkey3 other\n"),
		},
		// R3.3: -s N skips the first N characters.
		{
			Name:  "skip_chars",
			Args:  []string{"-s", "3"},
			Stdin: []byte("aaahello\nbbbhello\ncccworld\n"),
		},
		// R3.4: -w N compares only the first N characters.
		{
			Name:  "check_chars",
			Args:  []string{"-w", "3"},
			Stdin: []byte("abcXXX\nabcYYY\ndefZZZ\n"),
		},
		// Combined flags: -c -i.
		{
			Name:  "count_case_insensitive",
			Args:  []string{"-c", "-i"},
			Stdin: []byte("Hello\nhello\nHELLO\nworld\n"),
		},
		// Combined flags: -d -i.
		{
			Name:  "duplicates_case_insensitive",
			Args:  []string{"-d", "-i"},
			Stdin: []byte("Hello\nhello\nworld\n"),
		},
		// Combined flags: -f and -s together.
		{
			Name:  "skip_fields_and_chars",
			Args:  []string{"-f", "1", "-s", "2"},
			Stdin: []byte("aa XXhello\nbb YYhello\ncc ZZworld\n"),
		},
		// Empty input.
		{
			Name:  "empty_input",
			Args:  []string{},
			Stdin: []byte(""),
		},
		// Single line.
		{
			Name:  "single_line",
			Args:  []string{},
			Stdin: []byte("only\n"),
		},
		// All identical lines.
		{
			Name:  "all_identical",
			Args:  []string{},
			Stdin: []byte("same\nsame\nsame\n"),
		},
		// All identical lines with -c.
		{
			Name:  "all_identical_count",
			Args:  []string{"-c"},
			Stdin: []byte("same\nsame\nsame\n"),
		},
		// All identical lines with -d.
		{
			Name:  "all_identical_duplicates",
			Args:  []string{"-d"},
			Stdin: []byte("same\nsame\nsame\n"),
		},
		// All identical lines with -u (should produce no output).
		{
			Name:  "all_identical_unique",
			Args:  []string{"-u"},
			Stdin: []byte("same\nsame\nsame\n"),
		},
		// No duplicates.
		{
			Name:  "no_duplicates",
			Args:  []string{},
			Stdin: []byte("a\nb\nc\n"),
		},
		// No duplicates with -d (should produce no output).
		{
			Name:  "no_duplicates_d",
			Args:  []string{"-d"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// No duplicates with -u (all lines output).
		{
			Name:  "no_duplicates_u",
			Args:  []string{"-u"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// -z zero-terminated input/output.
		{
			Name:  "zero_terminated",
			Args:  []string{"-z"},
			Stdin: []byte("a\x00a\x00b\x00"),
		},
		// -z with -c.
		{
			Name:  "zero_terminated_count",
			Args:  []string{"-z", "-c"},
			Stdin: []byte("a\x00a\x00b\x00"),
		},
		// Long form flags.
		{
			Name:  "long_count",
			Args:  []string{"--count"},
			Stdin: []byte("a\na\nb\n"),
		},
		{
			Name:  "long_repeated",
			Args:  []string{"--repeated"},
			Stdin: []byte("a\na\nb\n"),
		},
		{
			Name:  "long_unique",
			Args:  []string{"--unique"},
			Stdin: []byte("a\na\nb\n"),
		},
		{
			Name:  "long_ignore_case",
			Args:  []string{"--ignore-case"},
			Stdin: []byte("A\na\nb\n"),
		},
		// Multiple runs with varying counts.
		{
			Name:  "multiple_runs",
			Args:  []string{"-c"},
			Stdin: []byte("x\nx\nx\ny\nz\nz\n"),
		},
		// Field skip with leading blanks.
		{
			Name:  "skip_fields_leading_blanks",
			Args:  []string{"-f", "1"},
			Stdin: []byte("  a hello\n  b hello\n  c world\n"),
		},
		// -w 0 compares zero characters (everything equal).
		{
			Name:  "check_chars_zero",
			Args:  []string{"-w", "0"},
			Stdin: []byte("abc\ndef\nghi\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
