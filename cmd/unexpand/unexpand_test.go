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
	refBin, err := exec.LookPath("gunexpand")
	if err != nil {
		t.Skipf("reference binary gunexpand not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		{
			Name:  "8_leading_spaces_to_tab",
			Stdin: []byte("        text\n"),
		},
		{
			Name:  "16_leading_spaces_to_two_tabs",
			Stdin: []byte("                text\n"),
		},
		{
			Name:  "4_leading_spaces_stay",
			Stdin: []byte("    text\n"),
		},
		{
			Name:  "non_leading_spaces_unchanged",
			Stdin: []byte("text        text\n"),
		},
		{
			Name:  "existing_tab_passthrough",
			Stdin: []byte("\ttext\n"),
		},
		{
			Name:  "spaces_then_tab_in_leading",
			Stdin: []byte("   \ttext\n"),
		},
		{
			Name:  "12_leading_spaces_partial",
			Stdin: []byte("            text\n"),
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
		{
			Name:  "tab_then_8_spaces",
			Stdin: []byte("\t        text\n"),
		},
		{
			Name:  "multiple_lines",
			Stdin: []byte("        first\n                second\n    third\n"),
		},
		{
			Name:  "trailing_spaces_unchanged",
			Stdin: []byte("text        \n"),
		},
		{
			Name:  "mixed_leading_tab_then_spaces",
			Stdin: []byte("\t    text\n"),
		},
		// R2.1: -a converts all runs of spaces, not just leading.
		{
			Name:  "a_flag_leading_spaces",
			Args:  []string{"-a"},
			Stdin: []byte("        text\n"),
		},
		{
			Name:  "a_flag_non_leading_spaces_to_tab",
			Args:  []string{"-a"},
			Stdin: []byte("a       b\n"),
		},
		{
			Name:  "a_flag_multiple_groups",
			Args:  []string{"-a"},
			Stdin: []byte("a       b       c\n"),
		},
		{
			Name:  "a_flag_mid_line_8_spaces",
			Args:  []string{"-a"},
			Stdin: []byte("ab      c\n"),
		},
		// R2.2: single space stays as space with -a.
		{
			Name:  "a_flag_single_space_preserved",
			Args:  []string{"-a"},
			Stdin: []byte("a b\n"),
		},
		{
			Name:  "a_flag_single_space_mid_line",
			Args:  []string{"-a"},
			Stdin: []byte("hello world\n"),
		},
		// R2.3: first non-whitespace does not stop conversion; -a processes entire line.
		{
			Name:  "a_flag_entire_line_processed",
			Args:  []string{"-a"},
			Stdin: []byte("x               y               z\n"),
		},
		{
			Name:  "a_flag_trailing_spaces",
			Args:  []string{"-a"},
			Stdin: []byte("text        \n"),
		},
		{
			Name:  "a_flag_mixed_runs",
			Args:  []string{"-a"},
			Stdin: []byte("  a     b  c       d\n"),
		},
		{
			Name:  "a_flag_all_flag_long",
			Args:  []string{"--all"},
			Stdin: []byte("a       b\n"),
		},
		// R3.1: -t N sets uniform tab stop interval.
		{
			Name:  "t_uniform_4_leading",
			Args:  []string{"-t", "4"},
			Stdin: []byte("    text\n"),
		},
		{
			Name:  "t_uniform_4_mid_line",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a   b\n"),
		},
		{
			Name:  "t_uniform_4_multiple_stops",
			Args:  []string{"-t", "4"},
			Stdin: []byte("        text\n"),
		},
		{
			Name:  "t_uniform_4_partial_spaces",
			Args:  []string{"-t", "4"},
			Stdin: []byte("  text\n"),
		},
		{
			Name:  "t_uniform_3",
			Args:  []string{"-t3"},
			Stdin: []byte("   x   y\n"),
		},
		{
			Name:  "t_tabs_equals_form",
			Args:  []string{"--tabs=4"},
			Stdin: []byte("    text\n"),
		},
		// R3.1: -t LIST sets absolute tab stop positions.
		{
			Name:  "t_list_4_8",
			Args:  []string{"-t", "4,8"},
			Stdin: []byte("    text\n"),
		},
		{
			Name:  "t_list_4_8_both_stops",
			Args:  []string{"-t", "4,8"},
			Stdin: []byte("        text\n"),
		},
		{
			Name:  "t_list_3_6_9",
			Args:  []string{"-t", "3,6,9"},
			Stdin: []byte("   text\n"),
		},
		{
			Name:  "t_list_mid_line",
			Args:  []string{"-t", "4,8"},
			Stdin: []byte("a   b   c\n"),
		},
		// R3.2: past last explicit stop in LIST, spaces kept as-is.
		{
			Name:  "t_list_past_last_stop_spaces_kept",
			Args:  []string{"-t", "4,8"},
			Stdin: []byte("        x        y\n"),
		},
		{
			Name:  "t_list_past_last_stop_no_conversion",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a   b   c   d\n"),
		},
		{
			Name:  "t_list_single_stop_past",
			Args:  []string{"-t", "3"},
			Stdin: []byte("            text\n"),
		},
		// R3.3: -t implies -a; all whitespace subject to conversion.
		{
			Name:  "t_implies_a_non_leading",
			Args:  []string{"-t", "8"},
			Stdin: []byte("text        more\n"),
		},
		{
			Name:  "t_implies_a_entire_line",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a   b   c   d\n"),
		},
		{
			Name:  "t_first_only_overrides_t",
			Args:  []string{"-t", "4", "--first-only"},
			Stdin: []byte("    a   b\n"),
		},
		{
			Name:  "t_list_implies_a",
			Args:  []string{"-t", "4,8,12"},
			Stdin: []byte("a   b       c\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
