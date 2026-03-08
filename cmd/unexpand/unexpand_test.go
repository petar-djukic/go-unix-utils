// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for the unexpand utility,
// comparing output against the GNU reference binary (gunexpand).
//
// Tests trace to prd025-unexpand R1, R2, R3, R4.
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
		// R1.1: leading 8 spaces become one tab.
		{
			Name:  "leading_spaces_to_tab",
			Args:  []string{},
			Stdin: []byte("        text\n"),
		},
		// R1.2: non-leading spaces unchanged in default mode.
		{
			Name:  "nonleading_spaces_unchanged",
			Args:  []string{},
			Stdin: []byte("a        b\n"),
		},
		// R1.3: partial space run (not reaching a tab stop) preserved.
		{
			Name:  "partial_spaces_preserved",
			Args:  []string{},
			Stdin: []byte("   text\n"),
		},
		// R1.3: no tabs possible with short spaces.
		{
			Name:  "no_tabs_possible",
			Args:  []string{},
			Stdin: []byte("a b c\n"),
		},
		// R1.4: existing tab in leading whitespace.
		{
			Name:  "existing_tab_in_leading",
			Args:  []string{},
			Stdin: []byte("\t    text\n"),
		},
		// R2.1: -a converts non-leading space runs at tab stops.
		{
			Name:  "all_mode",
			Args:  []string{"-a"},
			Stdin: []byte("a        b\n"),
		},
		// R2.1: -a with leading spaces.
		{
			Name:  "all_mode_leading",
			Args:  []string{"-a"},
			Stdin: []byte("        text\n"),
		},
		// R2.2: single space kept with -a.
		{
			Name:  "all_mode_single_space",
			Args:  []string{"-a"},
			Stdin: []byte("a b\n"),
		},
		// R2.3: -a processes entire line.
		{
			Name:  "all_mode_multiple_runs",
			Args:  []string{"-a"},
			Stdin: []byte("a        b        c\n"),
		},
		// R3.1: -t 4 uniform interval.
		{
			Name:  "custom_interval_4",
			Args:  []string{"-t", "4"},
			Stdin: []byte("    text\n"),
		},
		// R3.1: -t 4 with non-leading (implies -a).
		{
			Name:  "custom_interval_implies_all",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a    b\n"),
		},
		// R3.2: tab list with absolute stops.
		{
			Name:  "tab_list",
			Args:  []string{"-t", "4,8"},
			Stdin: []byte("    text\n"),
		},
		// R3.2: past last explicit stop, spaces kept.
		{
			Name:  "tab_list_past_all_stops",
			Args:  []string{"-t", "4,8"},
			Stdin: []byte("            text\n"),
		},
		// Empty input.
		{
			Name:  "empty_input",
			Args:  []string{},
			Stdin: []byte(""),
		},
		// Single newline.
		{
			Name:  "single_newline",
			Args:  []string{},
			Stdin: []byte("\n"),
		},
		// No trailing newline.
		{
			Name:  "no_trailing_newline",
			Args:  []string{},
			Stdin: []byte("        text"),
		},
		// Multiple lines.
		{
			Name:  "multiline",
			Args:  []string{},
			Stdin: []byte("        a\n        b\n"),
		},
		// Only spaces (exactly 8).
		{
			Name:  "only_spaces",
			Args:  []string{},
			Stdin: []byte("        \n"),
		},
		// Tab at end of line in default mode.
		{
			Name:  "spaces_at_end",
			Args:  []string{},
			Stdin: []byte("text        \n"),
		},
		// --all long option.
		{
			Name:  "long_all_option",
			Args:  []string{"--all"},
			Stdin: []byte("a        b\n"),
		},
		// --tabs=4 long option.
		{
			Name:  "tabs_equals_syntax",
			Args:  []string{"--tabs=4"},
			Stdin: []byte("    text\n"),
		},
		// 16 leading spaces become two tabs.
		{
			Name:  "two_tabs",
			Args:  []string{},
			Stdin: []byte("                text\n"),
		},
		// Mixed spaces and tabs in leading whitespace.
		{
			Name:  "mixed_leading_whitespace",
			Args:  []string{},
			Stdin: []byte("  \t  text\n"),
		},
		// -t 1 (every column is a tab stop).
		{
			Name:  "tab_interval_1",
			Args:  []string{"-t", "1"},
			Stdin: []byte("   text\n"),
		},
		// Large interval.
		{
			Name:  "large_interval",
			Args:  []string{"-t", "20"},
			Stdin: []byte("                    text\n"),
		},
		// --first-only flag (disables -a).
		{
			Name:  "first_only",
			Args:  []string{"-a", "--first-only"},
			Stdin: []byte("a        b\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
