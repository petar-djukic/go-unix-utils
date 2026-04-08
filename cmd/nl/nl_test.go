// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/nl. Implements srd022-nl R4.1, R4.2, R4.3, R4.4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skipf("reference binary gnl not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.1: Section delimiter lines are consumed and not written to output.
		// Header (\:\:\:), body (\:\:), and footer (\:) delimiters.
		{
			Name:  "section_delimiters_consumed",
			Args:  []string{"-h", "a", "-f", "a"},
			Stdin: []byte("\\:\\:\\:\nH1\n\\:\\:\nB1\n\\:\nF1\n"),
		},
		// R4.1: Delimiter with default styles (header=n, footer=n, body=t).
		{
			Name:  "section_delimiters_default_styles",
			Args:  []string{},
			Stdin: []byte("\\:\\:\\:\nheader\n\\:\\:\nbody\n\\:\nfooter\n"),
		},
		// R4.2: Header delimiter resets line counter to -v value (default 1).
		{
			Name:  "header_resets_counter",
			Args:  []string{"-b", "a"},
			Stdin: []byte("a\nb\n\\:\\:\\:\nc\nd\n"),
		},
		// R4.2: Header reset with custom start value (-v 10).
		{
			Name:  "header_resets_to_custom_start",
			Args:  []string{"-b", "a", "-v", "10"},
			Stdin: []byte("x\ny\n\\:\\:\\:\nz\nw\n"),
		},
		// R4.2: Body delimiter does NOT reset the counter.
		{
			Name:  "body_delimiter_no_reset",
			Args:  []string{"-b", "a"},
			Stdin: []byte("\\:\\:\\:\na\nb\n\\:\\:\nc\nd\n"),
		},
		// R4.3: -p suppresses line counter reset at logical page boundary.
		{
			Name:  "p_flag_suppresses_reset",
			Args:  []string{"-b", "a", "-p"},
			Stdin: []byte("a\nb\n\\:\\:\\:\nc\nd\n"),
		},
		// R4.3: -p with custom start value; counter continues after header.
		{
			Name:  "p_flag_with_custom_start",
			Args:  []string{"-b", "a", "-p", "-v", "5"},
			Stdin: []byte("x\ny\n\\:\\:\\:\nz\n"),
		},
		// R4.4: -l 2 with -b a; every 2nd consecutive empty line is numbered.
		{
			Name:  "join_blank_l2_style_a",
			Args:  []string{"-b", "a", "-l", "2"},
			Stdin: []byte("a\n\n\nb\n"),
		},
		// R4.4: -l 2 with -b t; empty lines remain unnumbered.
		{
			Name:  "join_blank_l2_style_t",
			Args:  []string{"-b", "t", "-l", "2"},
			Stdin: []byte("a\n\n\nb\n"),
		},
		// R4.4: Default -l 1 with -b a; all empty lines numbered.
		{
			Name:  "join_blank_default_style_a",
			Args:  []string{"-b", "a"},
			Stdin: []byte("a\n\nb\n"),
		},
		// R4.4: -l 3 with four consecutive empties.
		{
			Name:  "join_blank_l3_four_empties",
			Args:  []string{"-b", "a", "-l", "3"},
			Stdin: []byte("a\n\n\n\n\nb\n"),
		},
		// R4.1 + R4.2: test from test suite — header numbered with -h a.
		{
			Name:  "section_header_numbered",
			Args:  []string{"-h", "a"},
			Stdin: []byte("\\:\\:\\:\nheader line\n\\:\\:\nbody line\n"),
		},
		// R1.1/R1.2: Default body numbering (regression).
		{
			Name:  "default_body_numbering",
			Stdin: []byte("first\n\nsecond\n"),
		},
		// R2.1: -b a numbers all lines (regression).
		{
			Name:  "number_all_lines",
			Args:  []string{"-b", "a"},
			Stdin: []byte("x\n\ny\n"),
		},
		// R3.1/R3.2/R3.3: Format, width, separator (regression).
		{
			Name:  "format_ln_width_3_sep",
			Args:  []string{"-n", "ln", "-w", "3", "-s", ": "},
			Stdin: []byte("a\nb\n"),
		},
		// R3.4: Start value and increment (regression).
		{
			Name:  "start_and_increment",
			Args:  []string{"-v", "10", "-i", "5"},
			Stdin: []byte("p\nq\nr\n"),
		},
	}

	// R4.1: compare Go nl output against gnl byte-for-byte.
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
