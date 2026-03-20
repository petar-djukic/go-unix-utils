// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cut against gcut (GNU coreutils).
// Covers prd026-cut R1.1–R1.4: byte and character selection.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcut")
	if err != nil {
		t.Skip("reference binary gcut not in PATH")
	}
	tests := []testutils.DiffTest{
		{
			Name:  "byte_single_position",
			Args:  []string{"-b", "2"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "byte_range",
			Args:  []string{"-b", "2-4"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "byte_open_start",
			Args:  []string{"-b", "-3"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "byte_open_end",
			Args:  []string{"-b", "4-"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "byte_comma_list",
			Args:  []string{"-b", "1,3,5"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "byte_mixed_ranges",
			Args:  []string{"-b", "1-2,5-6"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "byte_overlapping_ranges",
			Args:  []string{"-b", "1-4,3-6"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "char_single_position",
			Args:  []string{"-c", "2"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "char_range",
			Args:  []string{"-c", "2-4"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "short_line_out_of_range",
			Args:  []string{"-b", "5-10"},
			Stdin: []byte("abc\n"),
		},
		{
			Name:  "very_short_line_all_out_of_range",
			Args:  []string{"-b", "10-20"},
			Stdin: []byte("abc\n"),
		},
		{
			Name:  "multiple_lines",
			Args:  []string{"-b", "2-3"},
			Stdin: []byte("abcdef\nghijkl\n"),
		},
		{
			Name:  "empty_line",
			Args:  []string{"-b", "1"},
			Stdin: []byte("\n"),
		},
		{
			Name:  "byte_flag_attached",
			Args:  []string{"-b1-3"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "char_flag_attached",
			Args:  []string{"-c1,4"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "stdin_dash",
			Args:  []string{"-b", "1-3", "-"},
			Stdin: []byte("abcdef\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
