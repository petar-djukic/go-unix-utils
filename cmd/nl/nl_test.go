// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/nl against gnl reference binary.
// Implements prd022-nl R1-R4.
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
		// R1.1: Default body numbering — non-empty lines numbered, empty lines pass through.
		{
			Name:  "default_body_numbering",
			Args:  []string{},
			Stdin: []byte("first\n\nsecond\n"),
		},
		// R1.2: Empty lines not numbered by default.
		{
			Name:  "empty_lines_unnumbered",
			Args:  []string{},
			Stdin: []byte("\n\n\n"),
		},
		// R1.3: Read from stdin when no file arguments.
		{
			Name:  "stdin_single_line",
			Args:  []string{},
			Stdin: []byte("only line\n"),
		},
		// R1.4: Continuous numbering across stdin.
		{
			Name:  "multiline_continuous",
			Args:  []string{},
			Stdin: []byte("a\nb\nc\nd\ne\n"),
		},
		// R2.1: -b a numbers all lines including empty.
		{
			Name:  "body_all",
			Args:  []string{"-b", "a"},
			Stdin: []byte("x\n\ny\n"),
		},
		// R2.1: -b t numbers non-empty lines (explicit).
		{
			Name:  "body_nonempty_explicit",
			Args:  []string{"-b", "t"},
			Stdin: []byte("x\n\ny\n"),
		},
		// R2.1: -b n numbers no lines.
		{
			Name:  "body_none",
			Args:  []string{"-b", "n"},
			Stdin: []byte("x\ny\n"),
		},
		// R2.1: -b pRE numbers lines matching regex.
		{
			Name:  "body_regex",
			Args:  []string{"-b", "p^[A-Z]"},
			Stdin: []byte("Hello\nworld\nGoodbye\nfriend\n"),
		},
		// R2.1: -ba combined form.
		{
			Name:  "body_all_combined",
			Args:  []string{"-ba"},
			Stdin: []byte("x\n\ny\n"),
		},
		// R3.1: -n ln left-justified format.
		{
			Name:  "format_ln",
			Args:  []string{"-n", "ln"},
			Stdin: []byte("a\nb\n"),
		},
		// R3.1: -n rn right-justified format (default).
		{
			Name:  "format_rn",
			Args:  []string{"-n", "rn"},
			Stdin: []byte("a\nb\n"),
		},
		// R3.1: -n rz right-justified with leading zeros.
		{
			Name:  "format_rz",
			Args:  []string{"-n", "rz"},
			Stdin: []byte("a\nb\n"),
		},
		// R3.2: -w sets field width.
		{
			Name:  "width_3",
			Args:  []string{"-w", "3"},
			Stdin: []byte("a\nb\n"),
		},
		// R3.1, R3.2, R3.3: Combined format, width, separator.
		{
			Name:  "format_ln_width_3_separator",
			Args:  []string{"-n", "ln", "-w", "3", "-s", ": "},
			Stdin: []byte("a\nb\n"),
		},
		// R3.3: -s custom separator.
		{
			Name:  "custom_separator",
			Args:  []string{"-s", "->"},
			Stdin: []byte("line\n"),
		},
		// R3.4: -v sets starting line number.
		{
			Name:  "start_number",
			Args:  []string{"-v", "10"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R3.4: -i sets increment.
		{
			Name:  "increment",
			Args:  []string{"-v", "10", "-i", "5"},
			Stdin: []byte("p\nq\nr\n"),
		},
		// R4.1: Section delimiters consumed; header/body transitions.
		{
			Name:  "section_delimiters",
			Args:  []string{"-h", "a"},
			Stdin: []byte("\\:\\:\\:\nheader line\n\\:\\:\nbody line\n"),
		},
		// R4.1: Footer delimiter.
		{
			Name:  "footer_section",
			Args:  []string{"-h", "a", "-f", "a"},
			Stdin: []byte("\\:\\:\\:\nH\n\\:\\:\nB\n\\:\nF\n"),
		},
		// R4.2: Header delimiter resets counter.
		{
			Name:  "header_resets_counter",
			Args:  []string{},
			Stdin: []byte("a\nb\n\\:\\:\\:\nc\nd\n"),
		},
		// R4.3: -p suppresses counter reset.
		{
			Name:  "no_renumber",
			Args:  []string{"-p"},
			Stdin: []byte("a\nb\n\\:\\:\\:\nc\nd\n"),
		},
		// R4.4: -l join blank lines.
		{
			Name:  "join_blank_2",
			Args:  []string{"-b", "a", "-l", "2"},
			Stdin: []byte("a\n\n\nb\n\nc\n"),
		},
		// R3.1: -n rz with -w 4.
		{
			Name:  "rz_width_4",
			Args:  []string{"-n", "rz", "-w", "4"},
			Stdin: []byte("x\ny\n"),
		},
		// Edge: empty input.
		{
			Name:  "empty_input",
			Args:  []string{},
			Stdin: []byte(""),
		},
		// Edge: single empty line.
		{
			Name:  "single_empty_line",
			Args:  []string{},
			Stdin: []byte("\n"),
		},
		// Combined short form: -ba -nln -w3.
		{
			Name:  "combined_short_flags",
			Args:  []string{"-ba", "-nln", "-w3"},
			Stdin: []byte("hello\nworld\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
