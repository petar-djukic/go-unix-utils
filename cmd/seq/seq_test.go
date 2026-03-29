// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/seq against gseq (GNU coreutils).
//
// Covers prd019-seq R4.4: single argument, two arguments, three arguments,
// descending sequence, floating-point sequence, equal-width, custom separator,
// format string, empty sequence, zero step error, and invalid format error.
package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeSeqErrors replaces the program name prefix so "gseq:" and "seq:"
// compare identically, and strips "Try ... --help" hint lines that gseq
// emits but the Go binary does not.
func normalizeSeqErrors(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	var result [][]byte
	for _, line := range lines {
		if bytes.Contains(line, []byte("--help")) &&
			bytes.Contains(line, []byte("Try")) {
			continue
		}
		// Replace "gseq:" prefix with "seq:"
		if bytes.HasPrefix(line, []byte("gseq:")) {
			line = append([]byte("seq:"), line[5:]...)
		}
		result = append(result, line)
	}
	return bytes.Join(result, []byte("\n"))
}

// discardAll blanks all output so tests check only exit code.
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gseq")
	if err != nil {
		t.Skip("reference binary gseq not in PATH")
	}

	errNorm := []testutils.NormalizeFunc{normalizeSeqErrors}

	tests := []testutils.DiffTest{
		// R4.4: single argument (seq 5)
		{
			Name:     "single_arg_5",
			Args:     []string{"5"},
			ExitCode: 0,
		},
		// R4.4: single argument (seq 1)
		{
			Name:     "single_arg_1",
			Args:     []string{"1"},
			ExitCode: 0,
		},
		// R4.4: two arguments (seq 2 5)
		{
			Name:     "two_args_2_5",
			Args:     []string{"2", "5"},
			ExitCode: 0,
		},
		// R4.4: three arguments (seq 1 2 10)
		{
			Name:     "three_args_1_2_10",
			Args:     []string{"1", "2", "10"},
			ExitCode: 0,
		},
		// R4.4: descending sequence (seq 5 -1 1)
		{
			Name:     "descending_5_to_1",
			Args:     []string{"5", "-1", "1"},
			ExitCode: 0,
		},
		// R4.4: floating-point sequence (seq 0.1 0.1 0.5)
		{
			Name:     "float_0.1_0.1_0.5",
			Args:     []string{"0.1", "0.1", "0.5"},
			ExitCode: 0,
		},
		// R4.4: equal-width (seq -w 8 12)
		{
			Name:     "equal_width_8_12",
			Args:     []string{"-w", "8", "12"},
			ExitCode: 0,
		},
		// R4.4: custom separator (seq -s ', ' 1 5)
		{
			Name:     "separator_comma_space",
			Args:     []string{"-s", ", ", "1", "5"},
			ExitCode: 0,
		},
		// R4.4: format string (seq -f '%.2f' 1 3)
		{
			Name:     "format_percent_2f",
			Args:     []string{"-f", "%.2f", "1", "3"},
			ExitCode: 0,
		},
		// R4.4: empty sequence (FIRST > LAST with positive STEP)
		{
			Name:     "empty_sequence",
			Args:     []string{"5", "1"},
			ExitCode: 0,
		},
		// R4.4: zero step error
		{
			Name:      "zero_step_error",
			Args:      []string{"1", "0", "5"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R4.4: invalid format error
		{
			Name:      "invalid_format_no_directive",
			Args:      []string{"-f", "hello", "1", "3"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R1.3: FIRST equals LAST prints one number
		{
			Name:     "first_equals_last",
			Args:     []string{"3", "3"},
			ExitCode: 0,
		},
		// R1.4: negative step with FIRST < LAST produces no output
		{
			Name:     "negative_step_empty",
			Args:     []string{"1", "-1", "5"},
			ExitCode: 0,
		},
		// R2.2: separator with --separator= long form
		{
			Name:     "separator_long_form",
			Args:     []string{"--separator=:", "1", "4"},
			ExitCode: 0,
		},
		// R3.1: format with %e specifier
		{
			Name:     "format_percent_e",
			Args:     []string{"-f", "%e", "1", "3"},
			ExitCode: 0,
		},
		// R3.1: format with %g specifier
		{
			Name:     "format_percent_g",
			Args:     []string{"-f", "%g", "1", "3"},
			ExitCode: 0,
		},
		// R3.3: equal-width with negative numbers
		{
			Name:     "equal_width_negative",
			Args:     []string{"-w", "-5", "5"},
			ExitCode: 0,
		},
		// R2.3: float precision from input
		{
			Name:     "float_precision_two_decimals",
			Args:     []string{"0.50", "0.25", "1.50"},
			ExitCode: 0,
		},
		// R3.2: format with two conversion specifiers is an error
		{
			Name:      "format_two_directives",
			Args:      []string{"-f", "%f %f", "1", "3"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R4.2: non-numeric argument
		{
			Name:      "non_numeric_argument",
			Args:      []string{"abc"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R1.1: seq with large step skipping
		{
			Name:     "large_step",
			Args:     []string{"1", "3", "10"},
			ExitCode: 0,
		},
		// R2.1: separator with empty string
		{
			Name:     "separator_empty_string",
			Args:     []string{"-s", "", "1", "3"},
			ExitCode: 0,
		},
		// --help exits 0
		{
			Name:      "help",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// --version exits 0
		{
			Name:      "version",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
