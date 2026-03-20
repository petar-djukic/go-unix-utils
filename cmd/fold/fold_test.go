// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd023-fold R1.1–R1.4, R2.1–R2.3.
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
		// R1: core line wrapping
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

		// R2.1: -w flag sets width
		{
			Name:  "w_flag_combined_bw",
			Args:  []string{"-bw5"},
			Stdin: []byte("abcdefghij"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "w_flag_large_width",
			Args:  []string{"-w", "200"},
			Stdin: []byte("short\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R2.2: column mode with tab expansion
		{
			Name:  "tab_within_width",
			Args:  []string{"-w", "9"},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "tab_causes_wrap",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abc\tdef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "tab_at_start",
			Args:  []string{"-w", "10"},
			Stdin: []byte("\tabcdef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "multiple_tabs",
			Args:  []string{"-w", "20"},
			Stdin: []byte("a\tb\tc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "backspace_handling",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcde\bfghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "carriage_return_resets",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abc\rdefghij\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R2.3: byte mode (-b)
		{
			Name:  "byte_mode_basic",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("abcdefghij"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "byte_mode_tab_no_expansion",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("a\tb\tcd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "byte_mode_combined_flag",
			Args:  []string{"-bw4"},
			Stdin: []byte("abcdefgh\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "byte_mode_with_newlines",
			Args:  []string{"-b", "-w", "3"},
			Stdin: []byte("abcdef\nghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestWidthValidation tests R2.1 error cases directly since error message
// format differs between GNU fold (includes strerror suffix) and this binary.
func TestWidthValidation(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	tests := []struct {
		name string
		args []string
	}{
		{"width_zero", []string{"-w", "0"}},
		{"width_negative", []string{"-w", "-1"}},
		{"width_non_numeric", []string{"-w", "abc"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command(goBin, tc.args...)
			err := cmd.Run()
			if err == nil {
				t.Fatalf("expected exit code 1 for args %v, got 0", tc.args)
			}
			exitErr, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatalf("expected ExitError, got %T: %v", err, err)
			}
			if exitErr.ExitCode() != 1 {
				t.Fatalf("expected exit code 1, got %d", exitErr.ExitCode())
			}
		})
	}
}
