// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/unexpand (prd025-unexpand R4).
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests comparing Go unexpand against gunexpand.
// R4: byte-for-byte comparison via RunDiffTests with LC_ALL=C.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// Graceful skip if gunexpand is not in PATH.
	refBin, err := exec.LookPath("gunexpand")
	if err != nil {
		t.Skipf("reference binary gunexpand not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: Eight leading spaces become one tab.
		{
			Name:  "unexpand_leading_spaces_to_tab",
			Stdin: []byte("        text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: Non-leading spaces are unchanged in default mode.
		{
			Name:  "unexpand_nonleading_spaces_unchanged",
			Stdin: []byte("a        b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -a converts non-leading space runs at tab stop boundaries.
		{
			Name:  "unexpand_all_mode",
			Args:  []string{"-a"},
			Stdin: []byte("a        b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: Partial spaces not reaching a tab stop are preserved.
		{
			Name:  "unexpand_partial_spaces_preserved",
			Stdin: []byte("   text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: -t 4 converts leading 4-space runs to tabs.
		{
			Name:  "unexpand_custom_interval",
			Args:  []string{"-t", "4"},
			Stdin: []byte("    text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// No tabs possible: input with only single spaces.
		{
			Name:  "unexpand_no_tabs_possible",
			Stdin: []byte("a b c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Empty input produces no output.
		{
			Name:  "unexpand_empty_input",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// Empty line passes through.
		{
			Name:  "unexpand_empty_line",
			Stdin: []byte("\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Tab-only line passes through.
		{
			Name:  "unexpand_tab_only",
			Stdin: []byte("\t\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Multiple lines: column resets at newline.
		{
			Name:  "unexpand_multiline",
			Stdin: []byte("        a\n        b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2: -a with leading and non-leading spaces.
		{
			Name:  "unexpand_all_leading_and_nonleading",
			Args:  []string{"-a"},
			Stdin: []byte("        a        b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: Mixed leading tabs and spaces.
		{
			Name:  "unexpand_mixed_leading_tab_spaces",
			Stdin: []byte("\t    text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// -t with comma-separated list.
		{
			Name:  "unexpand_tab_list",
			Args:  []string{"-t", "4,8"},
			Stdin: []byte("    text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// 16 leading spaces = 2 tabs at default stops.
		{
			Name:  "unexpand_two_tabs",
			Stdin: []byte("                text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// No trailing newline.
		{
			Name:  "unexpand_no_trailing_newline",
			Stdin: []byte("        text"),
			Env:   []string{"LC_ALL=C"},
		},
		// -a: single space between words not converted.
		{
			Name:  "unexpand_all_single_space",
			Args:  []string{"-a"},
			Stdin: []byte("a b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.3: -t implies -a — non-leading spaces also converted.
		{
			Name:  "unexpand_t_implies_a",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a       b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Spaces only line.
		{
			Name:  "unexpand_spaces_only",
			Stdin: []byte("        \n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
