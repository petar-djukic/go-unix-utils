// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for the expand utility,
// comparing output against the GNU reference binary (gexpand).
//
// Tests trace to prd024-expand R1, R2, R3.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gexpand")
	if err != nil {
		t.Skipf("reference binary gexpand not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: default tab expansion, single tab.
		{
			Name:  "default_single_tab",
			Args:  []string{},
			Stdin: []byte("a\tb\n"),
		},
		// R1.2: multiple consecutive tabs.
		{
			Name:  "multiple_tabs",
			Args:  []string{},
			Stdin: []byte("\t\tx\n"),
		},
		// R1.3: no tabs, passthrough.
		{
			Name:  "no_tabs",
			Args:  []string{},
			Stdin: []byte("hello world\n"),
		},
		// R2.1: -t 4 uniform interval.
		{
			Name:  "tab_interval_4",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a\tb\n"),
		},
		// R2.2: -t LIST with absolute tab stops.
		{
			Name:  "tab_list",
			Args:  []string{"-t", "1,5,9"},
			Stdin: []byte("\ta\tb\n"),
		},
		// R2.2: tab past last explicit stop replaced by single space.
		{
			Name:  "tab_past_last_stop",
			Args:  []string{"-t", "4"},
			Stdin: []byte("12345678\t9\n"),
		},
		// R1.4: multiline resets column counter.
		{
			Name:  "multiline",
			Args:  []string{},
			Stdin: []byte("\ta\n\tb\n"),
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
		// Tab at end of line.
		{
			Name:  "tab_at_end",
			Args:  []string{},
			Stdin: []byte("abc\t\n"),
		},
		// Multiple tabs with content between.
		{
			Name:  "tabs_with_content",
			Args:  []string{},
			Stdin: []byte("a\tb\tc\n"),
		},
		// Tab at column 8 (already at tab stop).
		{
			Name:  "tab_at_tab_stop",
			Args:  []string{},
			Stdin: []byte("12345678\tx\n"),
		},
		// -t 1 (every column is a tab stop).
		{
			Name:  "tab_interval_1",
			Args:  []string{"-t", "1"},
			Stdin: []byte("a\tb\tc\n"),
		},
		// -t with --tabs= syntax.
		{
			Name:  "tabs_equals_syntax",
			Args:  []string{"--tabs=4"},
			Stdin: []byte("a\tb\n"),
		},
		// No trailing newline.
		{
			Name:  "no_trailing_newline",
			Args:  []string{},
			Stdin: []byte("a\tb"),
		},
		// Only tabs.
		{
			Name:  "only_tabs",
			Args:  []string{},
			Stdin: []byte("\t\t\t\n"),
		},
		// -t with large interval.
		{
			Name:  "large_interval",
			Args:  []string{"-t", "20"},
			Stdin: []byte("x\ty\n"),
		},
		// Multiple lines with different tab positions.
		{
			Name:  "varied_tab_positions",
			Args:  []string{},
			Stdin: []byte("a\tb\nab\tc\nabc\td\n"),
		},
		// -t list with two stops.
		{
			Name:  "tab_list_two_stops",
			Args:  []string{"-t", "4,8"},
			Stdin: []byte("\t\tx\n"),
		},
		// Tab after text past last list stop.
		{
			Name:  "tab_list_past_all_stops",
			Args:  []string{"-t", "4,8"},
			Stdin: []byte("123456789\tx\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
