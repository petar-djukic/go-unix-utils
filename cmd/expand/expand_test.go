// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd024-expand R1.1–R1.4, R2.1–R2.4, R3.1–R3.4,
// R4.1–R4.3 against gexpand.
// R4.1: All tests compare Go expand output against gexpand via RunDiffTests.
// R4.2: Tests cover default tab expansion, -t N, -t LIST, no-tabs passthrough,
//        and multiple consecutive tabs.
// R4.3: LC_ALL=C is set by RunDiffTests to eliminate locale-dependent divergence.
package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeExpandStderr normalizes stderr error messages to account for
// program name differences (gexpand vs expand) and OS error message casing.
func normalizeExpandStderr(b []byte) []byte {
	s := string(b)
	s = strings.ReplaceAll(s, "gexpand:", "expand:")
	return bytes.ToLower([]byte(s))
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gexpand")
	if err != nil {
		t.Skipf("reference binary gexpand not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		// R1 tests: default tab expansion.
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
		// R2.1 tests: -t N uniform interval.
		{
			// R2.1: -t 4 sets tab stop every 4 columns.
			Name:  "expand_t4_single_tab",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a\tb\n"),
		},
		{
			// R2.1: -t 4 with consecutive tabs.
			Name:  "expand_t4_consecutive_tabs",
			Args:  []string{"-t", "4"},
			Stdin: []byte("\t\tx\n"),
		},
		{
			// R2.1: -t 1 replaces each tab with a single space.
			Name:  "expand_t1_minimal",
			Args:  []string{"-t", "1"},
			Stdin: []byte("a\tb\tc\n"),
		},
		{
			// R2.1: -tN combined form.
			Name:  "expand_t4_combined_form",
			Args:  []string{"-t4"},
			Stdin: []byte("a\tb\n"),
		},
		// R2.2 tests: -t LIST absolute positions.
		{
			// R2.2: Absolute positions 4,8,12.
			Name:  "expand_t_list_4_8_12",
			Args:  []string{"-t", "4,8,12"},
			Stdin: []byte("a\tb\tc\td\n"),
		},
		{
			// R2.2: Tab past last explicit stop gets single space.
			Name:  "expand_t_list_past_last",
			Args:  []string{"-t", "3,6"},
			Stdin: []byte("a\tb\tc\td\n"),
		},
		{
			// R2.2: Non-uniform positions 1,5,9.
			Name:  "expand_t_list_1_5_9",
			Args:  []string{"-t", "1,5,9"},
			Stdin: []byte("a\tb\tc\n"),
		},
		{
			// R2.2: Tab at exact stop position still advances to next stop.
			Name:  "expand_t_list_at_boundary",
			Args:  []string{"-t", "5,10"},
			Stdin: []byte("abcd\tx\n"),
		},
		// R2.3 tests: -t replaces default of 8.
		{
			// R2.3: -t replaces default tab stop interval of 8.
			Name:  "expand_t_replaces_default",
			Args:  []string{"-t", "3"},
			Stdin: []byte("\tx\n"),
		},
		{
			// R2.3: Multiple -t values accumulate into a list.
			Name:  "expand_multiple_t_accumulate",
			Args:  []string{"-t", "2", "-t", "4"},
			Stdin: []byte("a\tb\n"),
		},
		// R2.4 tests: single-value list = uniform interval.
		{
			// R2.4: -t 4 (single value) behaves as uniform interval.
			Name:  "expand_t_single_list_equals_interval",
			Args:  []string{"-t", "4"},
			Stdin: []byte("\t\t\tx\n"),
		},
		{
			// R2.1: --tabs= long form.
			Name:  "expand_tabs_long_form",
			Args:  []string{"--tabs=4"},
			Stdin: []byte("a\tb\n"),
		},
		{
			// R2.2: --tabs= with list.
			Name:  "expand_tabs_long_form_list",
			Args:  []string{"--tabs=4,8,12"},
			Stdin: []byte("a\tb\tc\td\n"),
		},
		// R3 tests: exit codes and error handling.
		{
			// R3.1: Successful processing exits 0.
			Name:  "expand_exit_0_success",
			Args:  []string{},
			Stdin: []byte("hello\tworld\n"),
		},
		{
			// R3.2: Nonexistent file exits 1.
			Name:      "expand_nonexistent_file_exit_1",
			Args:      []string{"/nonexistent/expand/test/file"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeExpandStderr},
		},
		{
			// R3.2: Nonexistent file with stdin ("-") continues processing.
			Name:      "expand_nonexistent_with_stdin",
			Args:      []string{"-", "/nonexistent/expand/test/file2"},
			Stdin:     []byte("a\tb\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeExpandStderr},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
