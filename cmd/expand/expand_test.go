// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/expand (prd024-expand R4).
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests comparing Go expand against gexpand.
// R4.1: byte-for-byte comparison via RunDiffTests.
// R4.3: LC_ALL=C set via DiffTest.Env.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// Graceful skip if gexpand is not in PATH.
	refBin, err := exec.LookPath("gexpand")
	if err != nil {
		t.Skipf("reference binary gexpand not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: Default tab expansion — tab at column 2 expands to 7 spaces.
		{
			Name:  "expand_default_single_tab",
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1, R1.2: Two consecutive tabs each advance to the next tab stop.
		{
			Name:  "expand_multiple_tabs",
			Stdin: []byte("\t\tx\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: Input with no tabs passes through unchanged.
		{
			Name:  "expand_no_tabs",
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -t 4 sets tab stops every 4 columns.
		{
			Name:  "expand_tab_interval_4",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: -t 1,5,9 sets absolute tab stops.
		{
			Name:  "expand_tab_list",
			Args:  []string{"-t", "1,5,9"},
			Stdin: []byte("\ta\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: Tab past the last explicit stop with uniform interval.
		{
			Name:  "expand_tab_past_last_stop",
			Args:  []string{"-t", "4"},
			Stdin: []byte("12345678\t9\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: Column counter resets at each newline.
		{
			Name:  "expand_multiline",
			Stdin: []byte("\ta\n\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: Multiple tabs within a line.
		{
			Name:  "expand_tabs_in_middle",
			Stdin: []byte("a\tb\tc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Empty input produces no output.
		{
			Name:  "expand_empty_input",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// Empty line passes through.
		{
			Name:  "expand_empty_line",
			Stdin: []byte("\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Tab only line.
		{
			Name:  "expand_tab_only",
			Stdin: []byte("\t\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: Last -t wins when given multiple times.
		{
			Name:  "expand_last_t_wins",
			Args:  []string{"-t", "4", "-t", "2"},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// -i flag: only leading tabs converted.
		{
			Name:  "expand_initial_only",
			Args:  []string{"-i"},
			Stdin: []byte("\ta\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// -i flag with no leading tabs.
		{
			Name:  "expand_initial_no_leading",
			Args:  []string{"-i"},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: Tab stop list with tabs past last explicit stop.
		{
			Name:  "expand_list_past_last",
			Args:  []string{"-t", "4,8"},
			Stdin: []byte("\t\t\tx\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// No trailing newline.
		{
			Name:  "expand_no_trailing_newline",
			Stdin: []byte("a\tb"),
			Env:   []string{"LC_ALL=C"},
		},
		// -t 1 makes every tab one space.
		{
			Name:  "expand_tab_1",
			Args:  []string{"-t", "1"},
			Stdin: []byte("\t\t\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
