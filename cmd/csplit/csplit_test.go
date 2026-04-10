// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main tests cmd/csplit via differential testing against gcsplit.
// Implements srd068-csplit R4.3, R4.4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

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
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
