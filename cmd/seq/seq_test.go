// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/seq.
//
// Implements: prd019-seq R1.5, R2.1, R2.2, R2.3
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const binGseq = "gseq"

// seqErrRe matches seq/gseq error prefix and normalizes the program name.
var seqErrRe = regexp.MustCompile(`(?m)^g?seq: `)

// seqTryRe matches the "Try '...(g)seq --help'" line and normalizes the
// program name and path. GNU uses the full binary path in the Try line.
var seqTryRe = regexp.MustCompile(`(?m)Try '[^']*g?seq --help'`)

// normalizeSeqErrors replaces the program name prefix in error messages so
// that "gseq: ..." and "seq: ..." compare equal.
func normalizeSeqErrors(b []byte) []byte {
	b = seqErrRe.ReplaceAll(b, []byte("seq: "))
	b = seqTryRe.ReplaceAll(b, []byte("Try 'seq --help'"))
	return b
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(binGseq)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", binGseq, err)
	}

	tests := []testutils.DiffTest{
		// R1.1: single argument — seq LAST.
		{
			Name: "r1.1_single_arg_seq_5",
			Args: []string{"5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: two arguments — seq FIRST LAST.
		{
			Name: "r1.1_two_args_seq_2_5",
			Args: []string{"2", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: three arguments — seq FIRST STEP LAST.
		{
			Name: "r1.1_three_args_seq_1_2_10",
			Args: []string{"1", "2", "10"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: descending sequence.
		{
			Name: "r1.2_descending_seq_5_neg1_1",
			Args: []string{"5", "-1", "1"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: FIRST equals LAST — single number.
		{
			Name: "r1.3_first_equals_last",
			Args: []string{"3", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: empty sequence — positive step, FIRST > LAST.
		{
			Name: "r1.4_empty_sequence_positive",
			Args: []string{"5", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: empty sequence — negative step, FIRST < LAST.
		{
			Name: "r1.4_empty_sequence_negative",
			Args: []string{"1", "-1", "5"},
			Env:  []string{"LC_ALL=C"},
		},

		// R1.5: zero step error.
		{
			Name:      "r1.5_zero_step_error",
			Args:      []string{"1", "0", "5"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSeqErrors},
		},
		// R1.5: zero step as 0.0.
		{
			Name:      "r1.5_zero_step_0.0",
			Args:      []string{"1", "0.0", "5"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSeqErrors},
		},
		// R1.5: zero step as -0.
		{
			Name:      "r1.5_zero_step_neg0",
			Args:      []string{"1", "-0", "5"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSeqErrors},
		},

		// R2.1: default separator is newline — verified by comparing stdout.
		{
			Name: "r2.1_default_newline_separator",
			Args: []string{"1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: trailing newline after last number.
		{
			Name: "r2.1_trailing_newline",
			Args: []string{"1"},
			Env:  []string{"LC_ALL=C"},
		},

		// R2.2: custom separator -s.
		{
			Name: "r2.2_separator_comma_space",
			Args: []string{"-s", ", ", "1", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: separator with -sFOO form.
		{
			Name: "r2.2_separator_short_form",
			Args: []string{"-s:", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: separator with --separator=STRING form.
		{
			Name: "r2.2_separator_long_form",
			Args: []string{"--separator= | ", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: empty separator.
		{
			Name: "r2.2_separator_empty",
			Args: []string{"-s", "", "1", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: separator with single number — no separator in output.
		{
			Name: "r2.2_separator_single_number",
			Args: []string{"-s", ", ", "1", "1"},
			Env:  []string{"LC_ALL=C"},
		},

		// R2.3: integer inputs produce integer output (no decimal point).
		{
			Name: "r2.3_integer_format",
			Args: []string{"1", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: floating-point inputs preserve precision.
		{
			Name: "r2.3_float_precision",
			Args: []string{"0.1", "0.1", "0.5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: mixed precision — uses maximum precision.
		{
			Name: "r2.3_mixed_precision",
			Args: []string{"1.0", "1", "3.0"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: high precision float.
		{
			Name: "r2.3_high_precision",
			Args: []string{"0.01", "0.01", "0.05"},
			Env:  []string{"LC_ALL=C"},
		},

		// R3.3: equal-width zero padding.
		{
			Name: "r3.3_equal_width",
			Args: []string{"-w", "8", "12"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: equal-width with wider range.
		{
			Name: "r3.3_equal_width_wider",
			Args: []string{"-w", "1", "100"},
			Env:  []string{"LC_ALL=C"},
		},

		// R4.2: invalid argument.
		{
			Name:      "r4.2_invalid_argument",
			Args:      []string{"abc"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSeqErrors},
		},

		// Combined: separator with equal-width.
		{
			Name: "combined_separator_and_equal_width",
			Args: []string{"-w", "-s", ", ", "8", "12"},
			Env:  []string{"LC_ALL=C"},
		},

		// Large step skipping values.
		{
			Name: "step_3",
			Args: []string{"1", "3", "10"},
			Env:  []string{"LC_ALL=C"},
		},
		// Descending by step 2.
		{
			Name: "descending_step_2",
			Args: []string{"10", "-2", "1"},
			Env:  []string{"LC_ALL=C"},
		},
		// Negative numbers in range.
		{
			Name: "negative_range",
			Args: []string{"-3", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// Equal-width with negative to positive.
		{
			Name: "equal_width_neg_to_pos",
			Args: []string{"-w", "-5", "5"},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
