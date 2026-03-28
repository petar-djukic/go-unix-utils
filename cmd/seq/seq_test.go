// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/seq against GNU gseq.
// Covers prd019-seq R4.1-R4.4 (exit codes and differential testing).
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes error messages between GNU gseq and Go seq.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?seq|gseq`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	operandErr := regexp.MustCompile(
		`(?:extra operand '[^']*'|missing operand)`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("seq"))
		b = tryHelp.ReplaceAll(b, nil)
		b = operandErr.ReplaceAll(b, []byte("operand error"))
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gseq")
	if err != nil {
		t.Skipf("reference binary gseq not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		// R4.4: single argument — seq 5 (FIRST=1, STEP=1).
		{
			Name: "single_arg_5",
			Args: []string{"5"},
		},
		// R4.4: two arguments — seq 2 5.
		{
			Name: "two_args_2_5",
			Args: []string{"2", "5"},
		},
		// R4.4: three arguments — seq 1 2 10.
		{
			Name: "three_args_1_2_10",
			Args: []string{"1", "2", "10"},
		},
		// R4.4: descending sequence — seq 5 -1 1.
		{
			Name: "descending_5_to_1",
			Args: []string{"5", "-1", "1"},
		},
		// R4.4: floating-point sequence — seq 0.1 0.1 0.5.
		{
			Name: "float_0.1_0.1_0.5",
			Args: []string{"0.1", "0.1", "0.5"},
		},
		// R4.4: equal-width — seq -w 8 12.
		{
			Name: "equal_width_8_12",
			Args: []string{"-w", "8", "12"},
		},
		// R4.4: custom separator — seq -s ', ' 1 5.
		{
			Name: "separator_comma_space",
			Args: []string{"-s", ", ", "1", "5"},
		},
		// R4.4: format string — seq -f '%.2f' 1 3.
		{
			Name: "format_percent_2f",
			Args: []string{"-f", "%.2f", "1", "3"},
		},
		// R4.4: empty sequence (FIRST > LAST with positive STEP).
		{
			Name: "empty_sequence",
			Args: []string{"5", "1"},
		},
		// R4.4: single value (FIRST equals LAST).
		{
			Name: "single_value",
			Args: []string{"3", "3"},
		},
		// R4.2/R4.4: zero step error.
		{
			Name:      "zero_step_error",
			Args:      []string{"1", "0", "5"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.2/R4.4: invalid format string — no specifier.
		{
			Name:      "invalid_format_no_specifier",
			Args:      []string{"-f", "hello", "1", "3"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.2/R4.4: non-numeric argument.
		{
			Name:      "non_numeric_arg",
			Args:      []string{"abc"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.2/R4.4: wrong argument count — no args.
		{
			Name:      "no_args_error",
			Args:      []string{},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.4: negative numbers — seq -3 -1.
		{
			Name: "negative_range",
			Args: []string{"-3", "-1"},
		},
		// R4.4: large range — seq 1 1000.
		{
			Name: "large_range_1_1000",
			Args: []string{"1", "1000"},
		},
		// R4.4: equal-width with negative numbers.
		{
			Name: "equal_width_negative",
			Args: []string{"-w", "-5", "5"},
		},
		// R4.4: format %g specifier.
		{
			Name: "format_percent_g",
			Args: []string{"-f", "%g", "1", "5"},
		},
		// R4.4: format %e specifier.
		{
			Name: "format_percent_e",
			Args: []string{"-f", "%e", "1", "3"},
		},
		// R4.4: descending float sequence.
		{
			Name: "descending_float",
			Args: []string{"1.0", "-0.5", "-1.0"},
		},
		// R4.4: separator with newline (--separator=).
		{
			Name: "separator_long_form",
			Args: []string{"--separator=:", "1", "5"},
		},
		// R4.4: equal-width with large numbers.
		{
			Name: "equal_width_large",
			Args: []string{"-w", "1", "100"},
		},
		// R4.2/R4.4: too many arguments.
		{
			Name:      "too_many_args",
			Args:      []string{"1", "2", "3", "4"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.4: seq 1 (single arg, smallest range).
		{
			Name: "single_arg_1",
			Args: []string{"1"},
		},
		// R4.4: float precision from input.
		{
			Name: "float_precision_input",
			Args: []string{"1.00", "1.00", "3.00"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
