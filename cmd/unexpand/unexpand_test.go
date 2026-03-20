// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd025-unexpand R1.1–R1.4, R2.1–R2.3: default leading
// whitespace conversion and -a all-whitespace conversion against gunexpand.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunexpand")
	if err != nil {
		t.Skipf("reference binary gunexpand not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		// R1.1: Leading spaces reaching a tab stop become a tab.
		{
			Name:  "eight_leading_spaces_to_tab",
			Stdin: []byte("        text\n"),
		},
		{
			Name:  "sixteen_leading_spaces_to_two_tabs",
			Stdin: []byte("                text\n"),
		},
		// R1.2: Non-leading whitespace unchanged in default mode.
		{
			Name:  "non_leading_spaces_unchanged",
			Stdin: []byte("a        b\n"),
		},
		{
			Name:  "non_leading_multiple_spaces_unchanged",
			Stdin: []byte("x   y   z\n"),
		},
		// R1.3: Partial spaces not reaching tab stop kept as spaces.
		{
			Name:  "three_leading_spaces_no_tab",
			Stdin: []byte("   text\n"),
		},
		{
			Name:  "nine_spaces_tab_plus_one_space",
			Stdin: []byte("         text\n"),
		},
		{
			Name:  "twelve_spaces_tab_plus_four",
			Stdin: []byte("            text\n"),
		},
		// R1.4: Existing tabs in leading whitespace advance column normally.
		{
			Name:  "existing_tab_in_leading",
			Stdin: []byte("\t   text\n"),
		},
		{
			Name:  "spaces_then_tab_in_leading",
			Stdin: []byte("   \t   text\n"),
		},
		{
			Name:  "two_tabs_in_leading",
			Stdin: []byte("\t\ttext\n"),
		},
		// Edge cases.
		{
			Name:  "empty_input",
			Stdin: []byte{},
		},
		{
			Name:  "newline_only",
			Stdin: []byte("\n"),
		},
		{
			Name:  "no_spaces_passthrough",
			Stdin: []byte("hello world\n"),
		},
		{
			Name:  "multiple_lines",
			Stdin: []byte("        line1\n   line2\n                line3\n"),
		},
		{
			Name:  "whitespace_only_line",
			Stdin: []byte("        \n"),
		},
		{
			Name:  "mixed_leading_trailing",
			Stdin: []byte("        text   \n"),
		},
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("        text\n"),
		},
		// R2.1: -a converts non-leading spaces where tabs align.
		{
			Name:  "a_flag_converts_non_leading_spaces",
			Args:  []string{"-a"},
			Stdin: []byte("a        b\n"),
		},
		{
			Name:  "a_flag_leading_spaces_still_converted",
			Args:  []string{"-a"},
			Stdin: []byte("        text\n"),
		},
		{
			Name:  "a_flag_multiple_groups",
			Args:  []string{"-a"},
			Stdin: []byte("x        y        z\n"),
		},
		// R2.2: Single space kept as space even with -a.
		{
			Name:  "a_flag_single_space_kept",
			Args:  []string{"-a"},
			Stdin: []byte("a b\n"),
		},
		{
			Name:  "a_flag_partial_spaces_kept",
			Args:  []string{"-a"},
			Stdin: []byte("a   b\n"),
		},
		// R2.3: -a processes the entire line past first non-whitespace.
		{
			Name:  "a_flag_processes_entire_line",
			Args:  []string{"-a"},
			Stdin: []byte("hello        world        end\n"),
		},
		{
			Name:  "a_flag_multiple_lines",
			Args:  []string{"-a"},
			Stdin: []byte("a        b\nc        d\n"),
		},
		// --all is synonym for -a.
		{
			Name:  "all_flag_synonym",
			Args:  []string{"--all"},
			Stdin: []byte("a        b\n"),
		},
		// R2.1/R2.3: -a with tabs in non-leading position.
		{
			Name:  "a_flag_tab_in_middle",
			Args:  []string{"-a"},
			Stdin: []byte("a\t   b\n"),
		},
		// R2.1: -a with whitespace-only line.
		{
			Name:  "a_flag_whitespace_only",
			Args:  []string{"-a"},
			Stdin: []byte("        \n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
