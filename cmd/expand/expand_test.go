// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd024-expand R1.1–R1.4 against gexpand.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gexpand")
	if err != nil {
		t.Skipf("reference binary gexpand not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		{
			// R1.1: One tab at column 1 expands to 7 spaces (advancing to column 9).
			Name:  "expand_default_single_tab",
			Args:  []string{},
			Stdin: []byte("a\tb\n"),
		},
		{
			// R1.1, R1.2: Two consecutive tabs each advance to the next tab stop.
			Name:  "expand_multiple_tabs",
			Args:  []string{},
			Stdin: []byte("\t\tx\n"),
		},
		{
			// R1.3: Input with no tabs passes through unchanged.
			Name:  "expand_no_tabs",
			Args:  []string{},
			Stdin: []byte("hello world\n"),
		},
		{
			// R1.4: Column counter resets to 1 at each newline.
			Name:  "expand_multiline",
			Args:  []string{},
			Stdin: []byte("\ta\n\tb\n"),
		},
		{
			// R1.1, R1.3: Tab after several characters.
			Name:  "expand_tab_mid_line",
			Args:  []string{},
			Stdin: []byte("abcde\tf\n"),
		},
		{
			// R1.2: Three consecutive tabs from column 1.
			Name:  "expand_three_consecutive_tabs",
			Args:  []string{},
			Stdin: []byte("\t\t\tx\n"),
		},
		{
			// R1.3: Empty input.
			Name:  "expand_empty_input",
			Args:  []string{},
			Stdin: []byte(""),
		},
		{
			// R1.4: Multiple blank lines with tabs.
			Name:  "expand_blank_lines",
			Args:  []string{},
			Stdin: []byte("\n\ta\n\n\tb\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
