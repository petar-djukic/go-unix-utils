// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements differential tests for prd068-csplit R1.1–R1.4.
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
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
