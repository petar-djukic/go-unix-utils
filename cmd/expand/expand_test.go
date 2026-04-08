// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/expand: differential testing against gexpand.
// Implements srd024-expand R4.1, R4.2, R4.3, R3.1, R3.2, R3.3, R3.4.
package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeStderr handles program name and error message case differences
// between the Go binary ("expand") and the GNU reference ("gexpand").
func normalizeStderr(data []byte) []byte {
	s := strings.ToLower(string(data))
	s = strings.ReplaceAll(s, "gexpand:", "expand:")
	return []byte(s)
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gexpand")
	if err != nil {
		t.Skipf("reference binary gexpand not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1, R1.3: default tab expansion (8-column interval)
		{
			Name:  "default_tab_expansion",
			Stdin: []byte("a\tb\n"),
		},
		// R1.2: multiple consecutive tabs advance independently
		{
			Name:  "multiple_consecutive_tabs",
			Stdin: []byte("\t\tindented\n"),
		},
		// R1.3: no tabs pass through unchanged
		{
			Name:  "no_tabs_passthrough",
			Stdin: []byte("hello world\n"),
		},
		// R1.4: newline resets column position
		{
			Name:  "multiple_lines",
			Stdin: []byte("a\tb\nc\td\n"),
		},
		// R1.1: tab at start of line
		{
			Name:  "tab_at_start",
			Stdin: []byte("\thello\n"),
		},
		// R2.1: uniform interval with -t N
		{
			Name:  "uniform_interval_t4",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a\tb\n"),
		},
		// R2.1: uniform interval with attached form -tN
		{
			Name:  "uniform_interval_attached",
			Args:  []string{"-t4"},
			Stdin: []byte("a\tb\n"),
		},
		// R2.2: tab stop list with multiple absolute positions
		{
			Name:  "tab_list_multiple_stops",
			Args:  []string{"-t", "1,5,9"},
			Stdin: []byte("a\tb\tc\n"),
		},
		// R2.2: tab past last explicit stop replaced by single space
		{
			Name:  "tab_past_last_stop",
			Args:  []string{"-t", "2,4"},
			Stdin: []byte("abcdef\tg\n"),
		},
		// R2.4: single-element list = uniform interval
		{
			Name:  "single_element_list_as_interval",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a\tb\tc\n"),
		},
		// R2.1: --tabs= long form
		{
			Name:  "long_form_tabs",
			Args:  []string{"--tabs=4"},
			Stdin: []byte("a\tb\n"),
		},
		// R2.2: tab list with --tabs long form
		{
			Name:  "long_form_tabs_list",
			Args:  []string{"--tabs=4,8,12"},
			Stdin: []byte("\t\t\tend\n"),
		},
		// R1.2: many consecutive tabs with list
		{
			Name:  "consecutive_tabs_with_list",
			Args:  []string{"-t", "3,6,9"},
			Stdin: []byte("\t\t\tx\n"),
		},
		// R2.1: tab stop interval of 1 (each tab = 1 space)
		{
			Name:  "interval_one",
			Args:  []string{"-t", "1"},
			Stdin: []byte("a\tb\tc\n"),
		},
		// R3.1: exit 0 on successful processing (explicit verification)
		{
			Name:  "exit_0_on_success",
			Stdin: []byte("no\ttabs\there\n"),
		},
		// R3.2: nonexistent file exits 1, error to stderr
		{
			Name:      "nonexistent_file_exit_1",
			Args:      []string{"nonexistent_file_xyz_42"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R3.2: nonexistent file with valid stdin continues processing
		{
			Name:      "nonexistent_then_stdin",
			Args:      []string{"nonexistent_file_xyz_42", "-"},
			Stdin:     []byte("a\tb\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
