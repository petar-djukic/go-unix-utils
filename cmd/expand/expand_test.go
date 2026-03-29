// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/expand against gexpand (GNU coreutils).
//
// Covers prd024-expand R1.1, R1.2, R1.3, R1.4.
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
		t.Skip("reference binary gexpand not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1: default tab expansion at every 8th column
		{
			Name:  "default_single_tab",
			Args:  []string{},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: multiple consecutive tabs advance independently
		{
			Name:  "multiple_consecutive_tabs",
			Args:  []string{},
			Stdin: []byte("\t\tx\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: non-tab characters pass through unchanged
		{
			Name:  "no_tabs_passthrough",
			Args:  []string{},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: newline resets column position
		{
			Name:  "multiline_column_reset",
			Args:  []string{},
			Stdin: []byte("\ta\n\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1, R1.3: tab at various column positions
		{
			Name:  "tab_mid_line",
			Args:  []string{},
			Stdin: []byte("abc\tdef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: tab at column 8 boundary
		{
			Name:  "tab_at_column_8",
			Args:  []string{},
			Stdin: []byte("12345678\tx\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: empty input
		{
			Name:  "empty_input",
			Args:  []string{},
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: multiple newlines
		{
			Name:  "only_newlines",
			Args:  []string{},
			Stdin: []byte("\n\n\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: tab at start of line
		{
			Name:  "tab_at_start",
			Args:  []string{},
			Stdin: []byte("\thello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: input with no trailing newline
		{
			Name:  "no_trailing_newline",
			Args:  []string{},
			Stdin: []byte("a\tb"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: stdin via '-' argument
		{
			Name:  "stdin_dash_arg",
			Args:  []string{"-"},
			Stdin: []byte("x\ty\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
