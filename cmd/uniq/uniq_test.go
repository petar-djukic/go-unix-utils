// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd028-uniq R1.1–R1.4: default adjacent-duplicate
// suppression, stdin/file input, case-sensitive comparison.
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
		{
			// R1.1: suppress adjacent duplicates, keep first of each run
			Name:  "adjacent_duplicates",
			Stdin: []byte("a\na\nb\na\n"),
		},
		{
			// R1.2: single-occurrence lines pass through
			Name:  "all_unique",
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R1.2: non-adjacent duplicates are unaffected
			Name:  "non_adjacent_duplicates",
			Stdin: []byte("a\nb\na\nb\n"),
		},
		{
			// R1.1: multiple runs of duplicates
			Name:  "multiple_runs",
			Stdin: []byte("x\nx\nx\ny\ny\nz\nz\nz\nz\n"),
		},
		{
			// R1.3: empty input produces empty output
			Name:  "empty_input",
			Stdin: []byte(""),
		},
		{
			// R1.4: case-sensitive comparison
			Name:  "case_sensitive",
			Stdin: []byte("A\na\nA\n"),
		},
		{
			// R1.1: single line
			Name:  "single_line",
			Stdin: []byte("hello\n"),
		},
		{
			// R1.1: all identical lines
			Name:  "all_identical",
			Stdin: []byte("foo\nfoo\nfoo\n"),
		},
		{
			// R1.4: lines with trailing spaces are distinct
			Name:  "trailing_spaces_differ",
			Stdin: []byte("abc\nabc \nabc\n"),
		},
		{
			// R1.1: blank lines are adjacent duplicates
			Name:  "blank_lines",
			Stdin: []byte("\n\n\na\n\n\n"),
		},
		{
			// R1.3: "-" means stdin
			Name:  "dash_means_stdin",
			Args:  []string{"-"},
			Stdin: []byte("a\na\nb\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
