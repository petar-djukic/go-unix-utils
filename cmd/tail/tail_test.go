// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd055-tail R1.1–R1.4 differential tests:
// line-count mode, stdin reading, -n flag, and +NUM offset.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtail")
	if err != nil {
		t.Skipf("reference binary gtail not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R1.1: default last 10 lines
			Name:  "default_last_10_lines",
			Stdin: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n13\n14\n15\n"),
		},
		{
			// R1.1: fewer than 10 lines prints all
			Name:  "fewer_than_10_lines",
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R1.2: -n 5 prints last 5 lines
			Name:  "n_flag_last_5",
			Args:  []string{"-n", "5"},
			Stdin: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"),
		},
		{
			// R1.2: --lines=3
			Name:  "lines_equals_3",
			Args:  []string{"--lines=3"},
			Stdin: []byte("a\nb\nc\nd\ne\nf\n"),
		},
		{
			// R1.2: -n1 attached form
			Name:  "n_attached_1",
			Args:  []string{"-n1"},
			Stdin: []byte("first\nsecond\nthird\n"),
		},
		{
			// R1.3: -n +3 prints from line 3 onward
			Name:  "n_plus_from_line_3",
			Args:  []string{"-n", "+3"},
			Stdin: []byte("1\n2\n3\n4\n5\n"),
		},
		{
			// R1.3: -n +1 prints all lines
			Name:  "n_plus_1_all_lines",
			Args:  []string{"-n", "+1"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R1.3: -n +100 on short input prints nothing
			Name:  "n_plus_beyond_end",
			Args:  []string{"-n", "+100"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R1.4: empty stdin
			Name:  "empty_stdin",
			Stdin: []byte{},
		},
		{
			// R1.4: stdin with - arg
			Name:  "dash_as_stdin",
			Args:  []string{"-"},
			Stdin: []byte("line1\nline2\nline3\n"),
		},
		{
			// R1.1: input without trailing newline
			Name:  "no_trailing_newline",
			Stdin: []byte("a\nb\nc"),
		},
		{
			// R1.2: -n 0 prints nothing
			Name:  "n_zero",
			Args:  []string{"-n", "0"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R1.1: exactly 10 lines
			Name:  "exactly_10_lines",
			Stdin: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
