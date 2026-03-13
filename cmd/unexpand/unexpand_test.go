// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd025-unexpand R1.1, R1.2, R1.3, R1.4
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests comparing the Go unexpand binary against the
// GNU reference binary (gunexpand) via pkg/testutils.RunDiffTests.
//
// R1.1-R1.4: Default leading-space-to-tab conversion with tab stop 8.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunexpand")
	if err != nil {
		t.Skipf("reference binary gunexpand not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R1.1: Eight leading spaces become one tab.
			Name:  "leading_8_spaces_to_tab",
			Stdin: []byte("        text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: Sixteen leading spaces become two tabs.
			Name:  "leading_16_spaces_to_two_tabs",
			Stdin: []byte("                text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.2: Non-leading spaces are not converted in default mode.
			Name:  "nonleading_spaces_unchanged",
			Stdin: []byte("a        b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.3: Partial run of spaces (fewer than 8) at start of line preserved.
			Name:  "partial_spaces_preserved",
			Stdin: []byte("   text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.3: Five leading spaces do not reach tab stop 8.
			Name:  "five_leading_spaces",
			Stdin: []byte("     text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1, R1.3: Ten leading spaces = one tab (8) + two spaces.
			Name:  "ten_leading_spaces",
			Stdin: []byte("          text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.4: Existing tab in leading whitespace followed by spaces.
			Name:  "existing_tab_then_spaces",
			Stdin: []byte("\t        text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.4: Tab followed by fewer spaces than next tab stop.
			Name:  "tab_then_partial_spaces",
			Stdin: []byte("\t   text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: Empty input passes through unchanged.
			Name:  "empty_input",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: Input with no spaces passes through unchanged.
			Name:  "no_spaces_passthrough",
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.2: Multiple lines, each processed independently.
			Name:  "multiple_lines",
			Stdin: []byte("        a\n        b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: No trailing newline — still converts.
			Name:  "no_trailing_newline",
			Stdin: []byte("        text"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.2: Single space in non-leading position unchanged.
			Name:  "single_nonleading_space",
			Stdin: []byte("a b c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.3: Seven leading spaces — not enough for a tab.
			Name:  "seven_leading_spaces",
			Stdin: []byte("       text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: Exactly eight spaces on a line by themselves.
			Name:  "eight_spaces_only",
			Stdin: []byte("        \n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.2: Stdin via "-" argument.
			Name:  "stdin_via_dash",
			Args:  []string{"-"},
			Stdin: []byte("        text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.2: Mixed: leading spaces on first line, non-leading on second.
			Name:  "mixed_leading_and_nonleading",
			Stdin: []byte("        a\nb        c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// Exit 1 on nonexistent file; normalize stderr (format differs).
			Name:     "exit_1_nonexistent_file",
			Args:     []string{"/nonexistent/unexpand_test_file"},
			ExitCode: 1,
			Env:      []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{
				clearStderr,
			},
		},
		{
			// Processing continues after error for remaining files.
			Name:     "continues_after_error",
			Args:     []string{"/nonexistent/unexpand_test_file", "-"},
			Stdin:    []byte("        text\n"),
			ExitCode: 1,
			Env:      []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{
				clearStderr,
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// clearStderr is a NormalizeFunc that replaces any non-empty output with empty
// bytes. Used for error-path tests where stderr message format differs between
// GNU unexpand and the Go implementation but exit code and stdout must still match.
func clearStderr(b []byte) []byte {
	return nil
}
