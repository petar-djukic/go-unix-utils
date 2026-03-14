// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/seq.
//
// Implements: prd019-seq R1.5, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3, R4.4
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

// normalizeVersionOutput replaces all version output with a canonical string.
// GNU gseq and Go seq produce different version strings; this normalizer
// ensures the differential test compares only that non-empty output was produced.
func normalizeVersionOutput(b []byte) []byte {
	if len(b) > 0 {
		return []byte("VERSION_OUTPUT\n")
	}
	return b
}

// normalizeHelpOutput replaces all help output with a canonical string.
// GNU gseq and Go seq produce different help text; this normalizer
// ensures the differential test compares only that non-empty output was produced.
func normalizeHelpOutput(b []byte) []byte {
	if len(b) > 0 {
		return []byte("HELP_OUTPUT\n")
	}
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
		// R1.1, R4.4: single argument — seq LAST.
		{
			Name: "r1.1_r4.4_single_arg_seq_5",
			Args: []string{"5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1, R4.4: two arguments — seq FIRST LAST.
		{
			Name: "r1.1_r4.4_two_args_seq_2_5",
			Args: []string{"2", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1, R4.4: three arguments — seq FIRST STEP LAST.
		{
			Name: "r1.1_r4.4_three_args_seq_1_2_10",
			Args: []string{"1", "2", "10"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2, R4.4: descending sequence.
		{
			Name: "r1.2_r4.4_descending_seq_5_neg1_1",
			Args: []string{"5", "-1", "1"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: FIRST equals LAST — single number.
		{
			Name: "r1.3_first_equals_last",
			Args: []string{"3", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4, R4.4: empty sequence — positive step, FIRST > LAST.
		{
			Name: "r1.4_r4.4_empty_sequence_positive",
			Args: []string{"5", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: empty sequence — negative step, FIRST < LAST.
		{
			Name: "r1.4_empty_sequence_negative",
			Args: []string{"1", "-1", "5"},
			Env:  []string{"LC_ALL=C"},
		},

		// R1.5, R4.4: zero step error.
		{
			Name:      "r1.5_r4.4_zero_step_error",
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

		// R2.2, R4.4: custom separator -s.
		{
			Name: "r2.2_r4.4_separator_comma_space",
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
		// R2.3, R4.4: floating-point inputs preserve precision.
		{
			Name: "r2.3_r4.4_float_precision",
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

		// R3.3, R4.4: equal-width zero padding.
		{
			Name: "r3.3_r4.4_equal_width",
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

		// R3.1: floating-point sequence with decimal increment.
		// AC1: seq 0.5 0.1 1.0 produces correct decimal precision.
		{
			Name: "r3.1_float_seq_0.5_0.1_1.0",
			Args: []string{"0.5", "0.1", "1.0"},
			Env:  []string{"LC_ALL=C"},
		},
		// AC2: seq 1 0.5 3 produces 1.0, 1.5, 2.0, 2.5, 3.0.
		{
			Name: "r3.1_float_seq_1_0.5_3",
			Args: []string{"1", "0.5", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: IEEE 754 rounding — seq 0.1 0.1 0.3 must include 0.3.
		// AC3: 0.3 must not be skipped due to floating-point rounding.
		{
			Name: "r3.3_ieee754_0.1_0.1_0.3",
			Args: []string{"0.1", "0.1", "0.3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1, R2.4: -f format string with floating-point output.
		// AC4: seq -f '%.3f' 1 0.5 2 applies format string.
		{
			Name: "r3.1_format_string_float",
			Args: []string{"-f", "%.3f", "1", "0.5", "2"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1, R4.4: -f format string with integer sequence.
		{
			Name: "r3.1_r4.4_format_string_integer",
			Args: []string{"-f", "%.2f", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: -f with %e format.
		{
			Name: "r3.1_format_string_e",
			Args: []string{"-f", "%e", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: -f with %g format.
		{
			Name: "r3.1_format_string_g",
			Args: []string{"-f", "%g", "0.5", "0.5", "2.0"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2, R4.4: invalid format — no conversion specifier.
		{
			Name:      "r3.2_r4.4_format_no_directive",
			Args:      []string{"-f", "hello", "1", "3"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSeqErrors},
		},
		// R3.2, R4.4: invalid format — unknown conversion specifier.
		{
			Name:      "r3.2_r4.4_format_unknown_directive",
			Args:      []string{"-f", "%d", "1", "3"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSeqErrors},
		},
		// R3.2: invalid format — too many conversion specifiers.
		{
			Name:      "r3.2_format_too_many_directives",
			Args:      []string{"-f", "%f %f", "1", "3"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSeqErrors},
		},
		// R3.4: -f and -w are mutually exclusive — GNU seq errors.
		{
			Name:      "r3.4_format_and_equal_width_error",
			Args:      []string{"-f", "%.2f", "-w", "1", "10"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSeqErrors},
		},
		// R3.1: --format= long form.
		{
			Name: "r3.1_format_long_form",
			Args: []string{"--format=%.1f", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: -fFORMAT combined form.
		{
			Name: "r3.1_format_combined_form",
			Args: []string{"-f%.1f", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: precision auto-detected from increment operand.
		{
			Name: "r3.2_precision_from_increment",
			Args: []string{"1", "0.25", "2"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: another IEEE 754 edge case.
		{
			Name: "r3.3_ieee754_1.0_0.1_1.3",
			Args: []string{"1.0", "0.1", "1.3"},
			Env:  []string{"LC_ALL=C"},
		},
		// Floating-point descending.
		{
			Name: "float_descending",
			Args: []string{"1.0", "-0.5", "-1.0"},
			Env:  []string{"LC_ALL=C"},
		},
		// Equal-width with float.
		{
			Name: "equal_width_float",
			Args: []string{"-w", "0.5", "0.1", "1.0"},
			Env:  []string{"LC_ALL=C"},
		},

		// R3.4: -w with descending float sequence.
		{
			Name: "r3.4_equal_width_descending_float",
			Args: []string{"-w", "1.0", "-0.1", "0.5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: large integer equal-width.
		{
			Name: "r3.4_equal_width_large",
			Args: []string{"-w", "1", "1000"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: boundary — last value exactly on boundary.
		{
			Name: "r3.4_boundary_exact_last",
			Args: []string{"0.0", "0.2", "1.0"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: boundary — floating-point rounding near boundary.
		{
			Name: "r3.4_boundary_float_rounding",
			Args: []string{"0.1", "0.2", "0.9"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: very small step.
		{
			Name: "r3.4_small_step",
			Args: []string{"0.00", "0.01", "0.05"},
			Env:  []string{"LC_ALL=C"},
		},

		// R4.1: --version prints version info to stdout and exits 0.
		{
			Name:      "r4.1_version_flag",
			Args:      []string{"--version"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeVersionOutput},
		},

		// R4.2: --help prints usage to stdout and exits 0.
		{
			Name:      "r4.2_help_flag",
			Args:      []string{"--help"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeHelpOutput},
		},

		// R4.3: missing operand — exit 1.
		{
			Name:      "r4.3_missing_operand",
			Args:      []string{},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSeqErrors},
		},
		// R4.3: extra operand — exit 1.
		{
			Name:      "r4.3_extra_operand",
			Args:      []string{"1", "1", "10", "20"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSeqErrors},
		},
		// R4.3: non-numeric argument — exit 1.
		{
			Name:      "r4.3_non_numeric_last",
			Args:      []string{"1", "xyz"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSeqErrors},
		},
		// R4.3: non-numeric first argument — exit 1.
		{
			Name:      "r4.3_non_numeric_first",
			Args:      []string{"foo", "5"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSeqErrors},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
