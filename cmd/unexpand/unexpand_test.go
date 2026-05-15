// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main_test

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunexpand")
	if err != nil {
		t.Skip("reference binary gunexpand not found")
	}
	tests := []testutils.DiffTest{
		// R1.1–R1.4: default (leading-only) behavior
		{
			Name:  "leading_8_spaces_become_tab",
			Stdin: []byte("        text\n"),
		},
		{
			Name:  "leading_4_spaces_stay",
			Stdin: []byte("    text\n"),
		},
		{
			Name:  "non_leading_spaces_unchanged_default",
			Stdin: []byte("a        b\n"),
		},
		{
			Name:  "mixed_leading_tab_and_spaces",
			Stdin: []byte("\t    text\n"),
		},
		{
			Name:  "leading_tab_preserved",
			Stdin: []byte("\ttext\n"),
		},
		{
			Name:  "empty_input",
			Stdin: []byte(""),
		},
		{
			Name:  "newline_only",
			Stdin: []byte("\n"),
		},
		{
			Name:  "no_whitespace",
			Stdin: []byte("hello\n"),
		},

		// R2.1: -a converts all runs of spaces to tabs at tab stops
		{
			Name:  "a_non_leading_8_spaces_to_tab",
			Args:  []string{"-a"},
			Stdin: []byte("a        b\n"),
		},
		{
			Name:  "a_leading_spaces_still_converted",
			Args:  []string{"-a"},
			Stdin: []byte("        text\n"),
		},
		{
			Name:  "a_multiple_runs_in_line",
			Args:  []string{"-a"},
			Stdin: []byte("a        b        c\n"),
		},
		{
			Name:  "a_spaces_at_various_positions",
			Args:  []string{"-a"},
			Stdin: []byte("ab      cd      ef\n"),
		},
		{
			Name:  "a_spaces_at_tab_boundary",
			Args:  []string{"-a"},
			Stdin: []byte("1234567 x\n"),
		},
		{
			Name:  "a_spaces_reaching_tab_stop_exactly",
			Args:  []string{"-a"},
			Stdin: []byte("12345   x\n"),
		},
		{
			Name:  "a_mixed_tabs_and_spaces_midline",
			Args:  []string{"-a"},
			Stdin: []byte("a\t        b\n"),
		},

		// R2.2: single space not reaching tab stop stays as space
		{
			Name:  "a_single_space_preserved",
			Args:  []string{"-a"},
			Stdin: []byte("a b\n"),
		},
		{
			Name:  "a_two_spaces_not_reaching_stop",
			Args:  []string{"-a"},
			Stdin: []byte("ab  cd\n"),
		},
		{
			Name:  "a_three_spaces_not_reaching_stop",
			Args:  []string{"-a"},
			Stdin: []byte("abc   def\n"),
		},
		{
			Name:  "a_single_space_between_words",
			Args:  []string{"-a"},
			Stdin: []byte("hello world\n"),
		},

		// R2.3: conversion continues past non-whitespace with -a
		{
			Name:  "a_processes_entire_line",
			Args:  []string{"-a"},
			Stdin: []byte("x        y        z\n"),
		},
		{
			Name:  "a_multiple_words_with_large_gaps",
			Args:  []string{"-a"},
			Stdin: []byte("aa              bb              cc\n"),
		},
		{
			Name:  "a_trailing_spaces",
			Args:  []string{"-a"},
			Stdin: []byte("text        \n"),
		},
		{
			Name:  "a_multiline",
			Args:  []string{"-a"},
			Stdin: []byte("a        b\nc        d\n"),
		},
		{
			Name:  "a_empty_input",
			Args:  []string{"-a"},
			Stdin: []byte(""),
		},
		{
			Name:  "a_only_spaces_16",
			Args:  []string{"-a"},
			Stdin: []byte("                \n"),
		},
		{
			Name:  "a_tab_stop_edge_7_spaces",
			Args:  []string{"-a"},
			Stdin: []byte("a       b\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
