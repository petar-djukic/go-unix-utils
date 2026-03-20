// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd025-unexpand R1.1–R1.4: default leading whitespace
// conversion against the GNU reference binary gunexpand.
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
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
