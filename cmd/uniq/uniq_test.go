// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/uniq: differential testing against guniq.
// Implements srd028-uniq R2.1, R2.2, R2.3, R2.4.
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
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
