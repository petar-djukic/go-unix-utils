// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gptx")
	if err != nil {
		t.Skipf("reference binary gptx not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		// R1.1: basic KWIC index generation
		{
			Name:  "basic_two_words",
			Args:  []string{},
			Stdin: []byte("hello world\n"),
		},
		{
			Name:  "empty_input",
			Args:  []string{},
			Stdin: []byte(""),
		},
		{
			Name:  "single_word",
			Args:  []string{},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "multiple_lines",
			Args:  []string{},
			Stdin: []byte("alpha beta\ngamma delta\n"),
		},
		{
			Name:  "three_words",
			Args:  []string{},
			Stdin: []byte("the quick fox\n"),
		},
		// R2.1: width control
		{
			Name:  "width_40",
			Args:  []string{"-w", "40"},
			Stdin: []byte("hello world\n"),
		},
		{
			Name:  "width_100",
			Args:  []string{"-w", "100"},
			Stdin: []byte("the quick brown fox\n"),
		},
		{
			Name:  "width_long_flag",
			Args:  []string{"--width=50"},
			Stdin: []byte("alpha beta gamma\n"),
		},
		// R2.2: case folding (-f)
		{
			Name:  "ignore_case",
			Args:  []string{"-f"},
			Stdin: []byte("Hello hello HELLO\n"),
		},
		{
			Name:  "ignore_case_sort_order",
			Args:  []string{"-f"},
			Stdin: []byte("Banana apple Cherry\n"),
		},
		{
			Name:  "ignore_case_long_flag",
			Args:  []string{"--ignore-case"},
			Stdin: []byte("Alpha alpha ALPHA\n"),
		},
		// R3.1: word regexp (-W)
		{
			Name:  "word_regexp_alpha",
			Args:  []string{"-W", "[a-zA-Z]+"},
			Stdin: []byte("hello, world! foo-bar\n"),
		},
		{
			Name:  "word_regexp_digits",
			Args:  []string{"-W", "[0-9]+"},
			Stdin: []byte("abc 123 def 456\n"),
		},
		// R4.1: auto reference (-A)
		{
			Name:  "auto_reference",
			Args:  []string{"-A"},
			Stdin: []byte("alpha beta\ngamma delta\n"),
		},
		{
			Name:  "auto_reference_with_width",
			Args:  []string{"-A", "-w", "80"},
			Stdin: []byte("one two three\nfour five six\n"),
		},
		// R4.2: references (-r)
		{
			Name:  "references_flag",
			Args:  []string{"-r"},
			Stdin: []byte("REF1 alpha beta\nREF2 gamma delta\n"),
		},
		// -S sentence regexp
		{
			Name:  "sentence_regexp_basic",
			Args:  []string{"-S", "[.?!]"},
			Stdin: []byte("hello world\n"),
		},
		{
			Name:  "sentence_regexp_long",
			Args:  []string{"--sentence-regexp=[.?!]"},
			Stdin: []byte("alpha beta\n"),
		},
		// Combinations
		{
			Name:  "combined_f_w",
			Args:  []string{"-f", "-w", "50"},
			Stdin: []byte("Apple banana Cherry\n"),
		},
		{
			Name:  "combined_A_w",
			Args:  []string{"-A", "-w", "80"},
			Stdin: []byte("one two three\nfour five six\n"),
		},
		{
			Name:  "combined_f_W",
			Args:  []string{"-f", "-W", "[a-z]+"},
			Stdin: []byte("Hello World 123 foo\n"),
		},
		{
			Name:  "combined_r_w",
			Args:  []string{"-r", "-w", "80"},
			Stdin: []byte("REF1 hello world\n"),
		},
		{
			Name:  "combined_f_A",
			Args:  []string{"-f", "-A"},
			Stdin: []byte("Alpha Beta\ngamma delta\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
