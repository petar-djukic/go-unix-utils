// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/cut: differential testing against gcut.
// Implements srd026-cut R1.1, R1.2, R1.3, R1.4.
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
		t.Skipf("reference binary gcut not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: extract single byte position
		{
			Name:  "byte_single_position",
			Args:  []string{"-b", "2"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.1: extract byte range N-M
		{
			Name:  "byte_range",
			Args:  []string{"-b", "2-4"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.1: extract bytes from N to end (N-)
		{
			Name:  "byte_range_open_end",
			Args:  []string{"-b", "3-"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.1: extract bytes from start to M (-M)
		{
			Name:  "byte_range_open_start",
			Args:  []string{"-b", "-3"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.1: comma-separated list of positions
		{
			Name:  "byte_comma_list",
			Args:  []string{"-b", "1,3,5"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.1: mixed ranges and positions
		{
			Name:  "byte_mixed_ranges",
			Args:  []string{"-b", "1,3-5"},
			Stdin: []byte("abcdefgh\n"),
		},
		// R1.2: -c is equivalent to -b under LC_ALL=C
		{
			Name:  "char_equivalent_to_byte",
			Args:  []string{"-c", "2-4"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.2: -c single position
		{
			Name:  "char_single_position",
			Args:  []string{"-c", "1"},
			Stdin: []byte("hello\n"),
		},
		// R1.3: newlines pass through to output
		{
			Name:  "newline_passthrough",
			Args:  []string{"-b", "1-2"},
			Stdin: []byte("ab\ncd\nef\n"),
		},
		// R1.3: multiple lines processed independently
		{
			Name:  "multiline_byte_selection",
			Args:  []string{"-b", "2"},
			Stdin: []byte("abc\ndef\nghi\n"),
		},
		// R1.4: short line produces only existing bytes
		{
			Name:  "short_line",
			Args:  []string{"-b", "1-10"},
			Stdin: []byte("abc\n"),
		},
		// R1.4: very short line with high range
		{
			Name:  "short_line_high_range",
			Args:  []string{"-b", "5-"},
			Stdin: []byte("ab\n"),
		},
		// R1.4: empty line produces empty output
		{
			Name:  "empty_line",
			Args:  []string{"-b", "1"},
			Stdin: []byte("\n"),
		},
		// R1.1: overlapping ranges merged correctly
		{
			Name:  "overlapping_ranges",
			Args:  []string{"-b", "1-3,2-5"},
			Stdin: []byte("abcdefgh\n"),
		},
		// R1.1: stdin via - argument
		{
			Name:  "stdin_explicit_dash",
			Args:  []string{"-b", "1-3", "-"},
			Stdin: []byte("hello\n"),
		},
		// R1.3: input without trailing newline
		{
			Name:  "no_trailing_newline",
			Args:  []string{"-b", "1-2"},
			Stdin: []byte("abcdef"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
