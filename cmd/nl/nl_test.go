// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for the nl utility,
// comparing output against the GNU reference binary (gnl).
//
// Tests trace to prd022-nl R1, R2, R3, R4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skipf("reference binary gnl not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1, R1.2: default body numbering — non-empty lines numbered, empty lines pass through.
		{
			Name:  "default_body_numbering",
			Args:  []string{},
			Stdin: []byte("first\n\nsecond\n"),
		},
		// R2.1: -b a numbers every line including empty.
		{
			Name:  "number_all_lines",
			Args:  []string{"-b", "a"},
			Stdin: []byte("x\n\ny\n"),
		},
		// R2.1: -b n numbers no lines.
		{
			Name:  "number_no_lines",
			Args:  []string{"-b", "n"},
			Stdin: []byte("hello\nworld\n"),
		},
		// R3.1: -n ln left-justified format.
		{
			Name:  "format_ln",
			Args:  []string{"-n", "ln"},
			Stdin: []byte("a\nb\n"),
		},
		// R3.1: -n rn right-justified format (default, explicit).
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
		// R3.1, R3.2, R3.3: combined format, width, and separator.
		{
			Name:  "format_ln_width_3_separator",
			Args:  []string{"-n", "ln", "-w", "3", "-s", ": "},
			Stdin: []byte("a\nb\n"),
		},
		// R3.2: custom width.
		{
			Name:  "custom_width",
			Args:  []string{"-w", "3"},
			Stdin: []byte("hello\nworld\n"),
		},
		// R3.3: custom separator.
		{
			Name:  "custom_separator",
			Args:  []string{"-s", "| "},
			Stdin: []byte("foo\nbar\n"),
		},
		// R3.4: start and increment.
		{
			Name:  "start_and_increment",
			Args:  []string{"-v", "10", "-i", "5"},
			Stdin: []byte("p\nq\nr\n"),
		},
		// R1.3: stdin input (no files).
		{
			Name:  "stdin_input",
			Args:  []string{},
			Stdin: []byte("line one\nline two\n"),
		},
		// R4.1, R4.2: section delimiters — header numbered with -h a.
		{
			Name:  "section_delimiters",
			Args:  []string{"-h", "a"},
			Stdin: []byte("\\:\\:\\:\nheader line\n\\:\\:\nbody line\n"),
		},
		// R4.1: footer section.
		{
			Name:  "footer_section",
			Args:  []string{"-f", "a"},
			Stdin: []byte("body line\n\\:\nfooter line\n"),
		},
		// R4.3: -p suppresses counter reset.
		{
			Name:  "no_reset_with_p",
			Args:  []string{"-p", "-h", "a"},
			Stdin: []byte("\\:\\:\\:\nheader\n\\:\\:\nbody\n\\:\\:\\:\nheader2\n"),
		},
		// R2.1: -b p regex numbering style.
		{
			Name:  "regex_style",
			Args:  []string{"-b", "p^[A-Z]"},
			Stdin: []byte("Hello\nworld\nFoo\nbar\n"),
		},
		// Empty input.
		{
			Name:  "empty_input",
			Args:  []string{},
			Stdin: []byte(""),
		},
		// Single line, no trailing newline.
		{
			Name:  "single_line_no_newline",
			Args:  []string{},
			Stdin: []byte("only"),
		},
		// R3.4: -v with rz format.
		{
			Name:  "start_rz_format",
			Args:  []string{"-v", "42", "-n", "rz"},
			Stdin: []byte("a\nb\n"),
		},
		// Multiple empty lines with default numbering.
		{
			Name:  "multiple_empty_lines",
			Args:  []string{},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		// R4.4: -l 2 join blank lines.
		{
			Name:  "join_blank_lines",
			Args:  []string{"-b", "a", "-l", "2"},
			Stdin: []byte("a\n\n\nb\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
