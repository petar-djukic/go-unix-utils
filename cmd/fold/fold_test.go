// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd023-fold R1.1–R1.4: core line wrapping.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfold")
	if err != nil {
		t.Skipf("reference binary gfold not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		{
			Name:  "short_line_passthrough",
			Stdin: []byte("short line\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "wrap_at_width_4",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefghij"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "line_exactly_at_width",
			Args:  []string{"-w", "5"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "trailing_newline_preserved",
			Args:  []string{"-w", "3"},
			Stdin: []byte("abcdef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "no_trailing_newline",
			Args:  []string{"-w", "3"},
			Stdin: []byte("abcde"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "repeated_wrapping",
			Args:  []string{"-w", "2"},
			Stdin: []byte("abcdefgh\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "empty_input",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "empty_line",
			Args:  []string{"-w", "5"},
			Stdin: []byte("\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "multiple_lines",
			Args:  []string{"-w", "3"},
			Stdin: []byte("abcd\nef\nghijkl\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "width_1",
			Args:  []string{"-w", "1"},
			Stdin: []byte("abc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "default_width_80_exact",
			Stdin: []byte("12345678901234567890123456789012345678901234567890123456789012345678901234567890\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "default_width_80_wrap",
			Stdin: []byte("123456789012345678901234567890123456789012345678901234567890123456789012345678901\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
