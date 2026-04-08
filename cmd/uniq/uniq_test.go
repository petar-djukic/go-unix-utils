// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/uniq: differential testing against guniq.
// Implements srd028-uniq R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R3.4.
package main_test

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
		// R1.1, R1.2: default deduplication
		{
			Name:  "default_adjacent_dedup",
			Stdin: []byte("a\na\nb\na\n"),
		},
		{
			Name:  "default_single_lines",
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			Name:  "default_all_same",
			Stdin: []byte("x\nx\nx\n"),
		},
		{
			Name:  "empty_input",
			Stdin: []byte(""),
		},
		{
			Name:  "single_line_no_newline",
			Stdin: []byte("abc"),
		},
		// R2.4: -c count prefix
		{
			Name:  "count_basic",
			Args:  []string{"-c"},
			Stdin: []byte("a\na\nb\n"),
		},
		{
			Name:  "count_all_unique",
			Args:  []string{"-c"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			Name:  "count_large_run",
			Args:  []string{"-c"},
			Stdin: []byte("x\nx\nx\nx\nx\nx\nx\nx\nx\nx\n"),
		},
		{
			Name:  "count_long",
			Args:  []string{"--count"},
			Stdin: []byte("a\na\na\nb\n"),
		},
		// R2.1: -d repeated only
		{
			Name:  "repeated_basic",
			Args:  []string{"-d"},
			Stdin: []byte("a\na\nb\n"),
		},
		{
			Name:  "repeated_none_duplicated",
			Args:  []string{"-d"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			Name:  "repeated_multiple_groups",
			Args:  []string{"-d"},
			Stdin: []byte("a\na\nb\nb\nc\n"),
		},
		{
			Name:  "repeated_long",
			Args:  []string{"--repeated"},
			Stdin: []byte("a\na\nb\n"),
		},
		// R2.2: -D all-repeated
		{
			Name:  "all_repeated_none",
			Args:  []string{"-D"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		{
			Name:  "all_repeated_none_explicit",
			Args:  []string{"--all-repeated=none"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		{
			Name:  "all_repeated_prepend",
			Args:  []string{"--all-repeated=prepend"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		{
			Name:  "all_repeated_separate",
			Args:  []string{"--all-repeated=separate"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		{
			Name:  "all_repeated_single_group",
			Args:  []string{"--all-repeated=separate"},
			Stdin: []byte("a\nb\nb\nc\n"),
		},
		{
			Name:  "all_repeated_no_duplicates",
			Args:  []string{"-D"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R2.3: -u unique only
		{
			Name:  "unique_basic",
			Args:  []string{"-u"},
			Stdin: []byte("a\na\nb\n"),
		},
		{
			Name:  "unique_all_duplicated",
			Args:  []string{"-u"},
			Stdin: []byte("a\na\nb\nb\n"),
		},
		{
			Name:  "unique_all_unique",
			Args:  []string{"-u"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			Name:  "unique_long",
			Args:  []string{"--unique"},
			Stdin: []byte("x\nx\ny\nz\nz\n"),
		},
		// R2.4 combined: -d -u produces no output
		{
			Name:  "repeated_and_unique_empty",
			Args:  []string{"-d", "-u"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R2.4: -c with -d
		{
			Name:  "count_with_repeated",
			Args:  []string{"-c", "-d"},
			Stdin: []byte("a\na\nb\nc\nc\nc\n"),
		},
		// R2.4: -c with -u
		{
			Name:  "count_with_unique",
			Args:  []string{"-c", "-u"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R3.1: -i case-insensitive comparison
		{
			Name:  "ignore_case_basic",
			Args:  []string{"-i"},
			Stdin: []byte("A\na\nb\n"),
		},
		{
			Name:  "ignore_case_all_same",
			Args:  []string{"-i"},
			Stdin: []byte("Hello\nhello\nHELLO\n"),
		},
		{
			Name:  "ignore_case_with_count",
			Args:  []string{"-i", "-c"},
			Stdin: []byte("A\na\nB\nb\nb\n"),
		},
		{
			Name:  "ignore_case_with_repeated",
			Args:  []string{"-i", "-d"},
			Stdin: []byte("A\na\nb\nC\nc\n"),
		},
		{
			Name:  "ignore_case_with_unique",
			Args:  []string{"-i", "-u"},
			Stdin: []byte("A\na\nb\nC\nc\n"),
		},
		{
			Name:  "ignore_case_long",
			Args:  []string{"--ignore-case"},
			Stdin: []byte("ABC\nabc\nDEF\n"),
		},
		// R3.2: -f N skip fields
		{
			Name:  "skip_fields_one",
			Args:  []string{"-f", "1"},
			Stdin: []byte("a foo\nb foo\nc bar\n"),
		},
		{
			Name:  "skip_fields_two",
			Args:  []string{"-f", "2"},
			Stdin: []byte("a b same\nc d same\ne f diff\n"),
		},
		{
			Name:  "skip_fields_with_tabs",
			Args:  []string{"-f", "1"},
			Stdin: []byte("x\taaa\ny\taaa\nz\tbbb\n"),
		},
		{
			Name:  "skip_fields_more_than_exist",
			Args:  []string{"-f", "5"},
			Stdin: []byte("a b\nc d\ne\n"),
		},
		{
			Name:  "skip_fields_with_count",
			Args:  []string{"-f", "1", "-c"},
			Stdin: []byte("1 abc\n2 abc\n3 def\n"),
		},
		{
			Name:  "skip_fields_long",
			Args:  []string{"--skip-fields=1"},
			Stdin: []byte("a same\nb same\nc diff\n"),
		},
		// R3.3: -s N skip chars
		{
			Name:  "skip_chars_basic",
			Args:  []string{"-s", "1"},
			Stdin: []byte("xabc\nyabc\nzdef\n"),
		},
		{
			Name:  "skip_chars_three",
			Args:  []string{"-s", "3"},
			Stdin: []byte("xxxsame\nyyysame\nzzzdiff\n"),
		},
		{
			Name:  "skip_chars_more_than_length",
			Args:  []string{"-s", "100"},
			Stdin: []byte("short\ntiny\n"),
		},
		{
			Name:  "skip_chars_with_count",
			Args:  []string{"-s", "2", "-c"},
			Stdin: []byte("aafoo\nbbfoo\nccbar\n"),
		},
		{
			Name:  "skip_chars_long",
			Args:  []string{"--skip-chars=2"},
			Stdin: []byte("xxhello\nyyhello\nzzworld\n"),
		},
		// R3.4: -f, -s, -i composition
		{
			Name:  "skip_fields_and_chars",
			Args:  []string{"-f", "1", "-s", "2"},
			Stdin: []byte("a xxhello\nb yyhello\nc zzworld\n"),
		},
		{
			Name:  "skip_fields_and_ignore_case",
			Args:  []string{"-f", "1", "-i"},
			Stdin: []byte("a ABC\nb abc\nc DEF\n"),
		},
		{
			Name:  "skip_chars_and_ignore_case",
			Args:  []string{"-s", "1", "-i"},
			Stdin: []byte("xABC\nyabc\nzDEF\n"),
		},
		{
			Name:  "all_three_combined",
			Args:  []string{"-f", "1", "-s", "1", "-i"},
			Stdin: []byte("a xHELLO\nb yhello\nc zWORLD\n"),
		},
		{
			Name:  "skip_fields_and_chars_with_repeated",
			Args:  []string{"-f", "1", "-s", "1", "-d"},
			Stdin: []byte("a xsame\nb ysame\nc zdiff\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
