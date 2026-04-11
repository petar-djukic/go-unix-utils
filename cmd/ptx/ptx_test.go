// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gptx")
	if err != nil {
		t.Skipf("reference binary gptx not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		{
			Name:  "single_line",
			Stdin: []byte("the quick brown fox\n"),
		},
		{
			Name:  "empty_input",
			Stdin: []byte(""),
		},
		{
			Name:  "multiple_lines",
			Stdin: []byte("hello world\nfoo bar baz\n"),
		},
		{
			Name:  "ignore_case_flag",
			Args:  []string{"-f"},
			Stdin: []byte("Alpha beta Gamma\n"),
		},
		{
			Name:  "single_word_line",
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "width_flag",
			Args:  []string{"-w", "40"},
			Stdin: []byte("one two three four\n"),
		},
		{
			Name:  "auto_reference_stdin",
			Args:  []string{"-A"},
			Stdin: []byte("hello world\nfoo bar\n"),
		},
		{
			Name:  "auto_reference_right",
			Args:  []string{"-A", "-R"},
			Stdin: []byte("hello world\nfoo bar\n"),
		},
		{
			Name:  "sentence_refs",
			Args:  []string{"-r"},
			Stdin: []byte("p1 hello world\np2 foo bar\n"),
		},
		{
			Name:  "sentence_refs_right",
			Args:  []string{"-r", "-R"},
			Stdin: []byte("p1 hello world\np2 foo bar\n"),
		},
		{
			Name:  "auto_ref_single_line",
			Args:  []string{"-A"},
			Stdin: []byte("alpha beta gamma\n"),
		},
		{
			Name:  "right_ref_width",
			Args:  []string{"-A", "-R", "-w", "60"},
			Stdin: []byte("one two three\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
