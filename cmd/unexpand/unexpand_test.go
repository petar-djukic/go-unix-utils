// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/unexpand against gunexpand.
// Covers prd025-unexpand R4.1 (flag combinations) and R4.2 (edge cases).
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
		// R4.1: differential tests for all flag combinations.
		{
			Name:  "leading_spaces_default",
			Args:  []string{},
			Stdin: []byte("        text\n"),
		},
		{
			Name:  "non_leading_spaces_default",
			Args:  []string{},
			Stdin: []byte("a        b\n"),
		},
		{
			Name:  "all_flag_leading",
			Args:  []string{"-a"},
			Stdin: []byte("        text\n"),
		},
		{
			Name:  "all_flag_non_leading",
			Args:  []string{"-a"},
			Stdin: []byte("a        b\n"),
		},
		{
			Name:  "first_only_flag",
			Args:  []string{"--first-only"},
			Stdin: []byte("        text        more\n"),
		},
		{
			Name:  "tabs_flag_4",
			Args:  []string{"--tabs=4"},
			Stdin: []byte("    text\n"),
		},
		{
			Name:  "tabs_flag_4_non_leading",
			Args:  []string{"--tabs=4"},
			Stdin: []byte("a    b\n"),
		},
		{
			Name:  "short_t_flag",
			Args:  []string{"-t", "4"},
			Stdin: []byte("    text    more\n"),
		},
		{
			Name:  "tabs_flag_implies_all",
			Args:  []string{"-t4"},
			Stdin: []byte("    text    more\n"),
		},
		{
			Name:  "tabs_list",
			Args:  []string{"-t", "4,8,12"},
			Stdin: []byte("    text    more        end\n"),
		},
		{
			Name:  "all_and_tabs_combined",
			Args:  []string{"-a", "--tabs=4"},
			Stdin: []byte("    text    more\n"),
		},
		// R4.2: edge cases.
		{
			Name:  "empty_input",
			Args:  []string{},
			Stdin: []byte(""),
		},
		{
			Name:  "tabs_only",
			Args:  []string{},
			Stdin: []byte("\t\t\t\n"),
		},
		{
			Name:  "spaces_only",
			Args:  []string{},
			Stdin: []byte("        \n"),
		},
		{
			Name:  "mixed_tabs_spaces",
			Args:  []string{},
			Stdin: []byte("\t    text\n"),
		},
		{
			Name:  "single_space_not_at_stop",
			Args:  []string{"-a"},
			Stdin: []byte("abc d\n"),
		},
		{
			Name:  "multiple_lines",
			Args:  []string{"-a"},
			Stdin: []byte("        line1\n        line2\na        b\n"),
		},
		{
			Name:  "no_newline_at_end",
			Args:  []string{},
			Stdin: []byte("        text"),
		},
		{
			Name:  "partial_tab_stop_spaces",
			Args:  []string{},
			Stdin: []byte("   text\n"),
		},
		{
			Name:  "exact_two_tab_stops",
			Args:  []string{},
			Stdin: []byte("                text\n"),
		},
		{
			Name:  "all_mode_mixed_content",
			Args:  []string{"-a"},
			Stdin: []byte("hello        world        foo\n"),
		},
		{
			Name:  "tabs_list_past_last_stop",
			Args:  []string{"-t", "4,8"},
			Stdin: []byte("            beyond stops\n"),
		},
		{
			Name:  "stdin_dash_arg",
			Args:  []string{"-"},
			Stdin: []byte("        text\n"),
		},
		{
			Name:  "spaces_only_all_mode",
			Args:  []string{"-a"},
			Stdin: []byte("                \n"),
		},
		{
			Name:  "tab_size_2",
			Args:  []string{"-t2"},
			Stdin: []byte("  a  b  c\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
