// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/seq against gseq (GNU coreutils).
// Implements prd019-seq R1.1-R1.5, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4 test coverage.
package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gseq")
	if err != nil {
		t.Skipf("reference binary gseq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: single argument form — seq LAST.
		{
			Name: "R1.1_single_arg_seq_5",
			Args: []string{"5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: single argument, seq 1 prints just "1".
		{
			Name: "R1.1_single_arg_seq_1",
			Args: []string{"1"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: two argument form — seq FIRST LAST.
		{
			Name: "R1.1_two_args_seq_3_7",
			Args: []string{"3", "7"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: three argument form — seq FIRST STEP LAST.
		{
			Name: "R1.1_three_args_seq_1_2_10",
			Args: []string{"1", "2", "10"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: descending sequence with negative step.
		{
			Name: "R1.2_descending_5_-1_1",
			Args: []string{"5", "-1", "1"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: step of 3.
		{
			Name: "R1.2_step_of_3",
			Args: []string{"1", "3", "10"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: FIRST equals LAST — prints exactly one number.
		{
			Name: "R1.3_first_equals_last",
			Args: []string{"5", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: FIRST equals LAST with explicit step.
		{
			Name: "R1.3_first_equals_last_explicit_step",
			Args: []string{"3", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: empty sequence — positive step, FIRST > LAST.
		{
			Name: "R1.4_empty_positive_step",
			Args: []string{"10", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: empty sequence — negative step, FIRST < LAST.
		{
			Name: "R1.4_empty_negative_step",
			Args: []string{"1", "-1", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: empty sequence — seq 0 (FIRST=1 > LAST=0).
		{
			Name: "R1.4_empty_seq_0",
			Args: []string{"0"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1/R1.4: floating-point sequence.
		{
			Name: "R1.4_float_0.5_0.5_2.5",
			Args: []string{"0.5", "0.5", "2.5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: floating-point with two args.
		{
			Name: "R1.4_float_1.0_3",
			Args: []string{"1.0", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: floating-point with higher precision.
		{
			Name: "R1.4_float_0.50_0.25_1.00",
			Args: []string{"0.50", "0.25", "1.00"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: last value exactly reachable by step.
		{
			Name: "R1.2_exact_last_seq_2_2_10",
			Args: []string{"2", "2", "10"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: last value not exactly reachable by step.
		{
			Name: "R1.2_inexact_last_seq_1_3_10",
			Args: []string{"1", "3", "10"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: descending with step -2.
		{
			Name: "R1.2_descending_step_-2",
			Args: []string{"10", "-2", "1"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: large range.
		{
			Name: "R1.1_large_range_1_100",
			Args: []string{"100"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: negative first and last via --.
		{
			Name: "R1.4_negative_range_via_separator",
			Args: []string{"--", "-3", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.5/AC1: floating-point sequence 0.5 0.1 1.0.
		{
			Name: "R1.5_float_seq_0.5_0.1_1.0",
			Args: []string{"0.5", "0.1", "1.0"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2/AC3: custom separator -s ', '.
		{
			Name: "R2.2_separator_comma_space",
			Args: []string{"-s", ", ", "1", "4"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: custom separator -s ':'.
		{
			Name: "R2.2_separator_colon",
			Args: []string{"-s", ":", "1", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: empty separator.
		{
			Name: "R2.2_separator_empty",
			Args: []string{"-s", "", "1", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: long option --separator=.
		{
			Name: "R2.2_long_separator",
			Args: []string{"--separator=:", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1/AC2: format flag -f '%.2f'.
		{
			Name: "R3.1_format_percent_2f",
			Args: []string{"-f", "%.2f", "1", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: format flag with %e.
		{
			Name: "R3.1_format_percent_e",
			Args: []string{"-f", "%e", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: format flag with %g.
		{
			Name: "R3.1_format_percent_g",
			Args: []string{"-f", "%g", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: format with width and precision.
		{
			Name: "R3.1_format_width_precision",
			Args: []string{"-f", "%010.3f", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: long option --format=.
		{
			Name: "R3.1_long_format",
			Args: []string{"--format=%.2f", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: format with surrounding text.
		{
			Name: "R3.1_format_with_text",
			Args: []string{"-f", "num_%g_end", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3/AC4: combined -f and -s flags.
		{
			Name: "R2.3_combined_format_separator",
			Args: []string{"-f", "%.1f", "-s", ":", "1", "0.5", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: -f with float args.
		{
			Name: "R2.3_format_with_float_args",
			Args: []string{"-f", "%.3f", "0.5", "0.5", "2.0"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: separator with single number.
		{
			Name: "R2.2_separator_single_number",
			Args: []string{"-s", ":", "1", "1"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: -f combined in short form -f%.2f.
		{
			Name: "R3.1_format_short_combined",
			Args: []string{"-f%.2f", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: -s combined in short form -s:.
		{
			Name: "R2.2_separator_short_combined",
			Args: []string{"-s:", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.5: float sequence with small increments.
		{
			Name: "R1.5_float_0.1_0.1_0.5",
			Args: []string{"0.1", "0.1", "0.5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.4: large integer near 2^32.
		{
			Name: "R2.4_large_int_near_2pow32",
			Args: []string{"4294967294", "1", "4294967296"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.4: large integer near 2^40.
		{
			Name: "R2.4_large_int_near_2pow40",
			Args: []string{"1099511627774", "1", "1099511627776"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: equal-width with -w, single digit to double digit.
		{
			Name: "R3.3_equal_width_8_12",
			Args: []string{"-w", "8", "12"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: equal-width with --equal-width long option.
		{
			Name: "R3.3_equal_width_long_option_1_10",
			Args: []string{"--equal-width", "1", "10"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: equal-width with 3-digit numbers.
		{
			Name: "R3.3_equal_width_98_101",
			Args: []string{"-w", "98", "101"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: -f and -w are mutually exclusive — error.
		{
			Name:      "R3.4_format_and_equal_width_error",
			Args:      []string{"-w", "-f", "%.2f", "1", "3"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4.4: exact two-argument form (seq 2 5).
		{
			Name: "R4.4_two_args_seq_2_5",
			Args: []string{"2", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.4: exact custom separator (seq -s ', ' 1 5).
		{
			Name: "R4.4_separator_comma_space_1_5",
			Args: []string{"-s", ", ", "1", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.4: exact format string (seq -f '%.2f' 1 3).
		{
			Name: "R4.4_format_percent_2f_1_3",
			Args: []string{"-f", "%.2f", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.4: boundary — step exactly spans range (two numbers).
		{
			Name: "R4.4_boundary_step_equals_range",
			Args: []string{"1", "4", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.4: boundary — negative to positive range.
		{
			Name: "R4.4_boundary_negative_to_positive",
			Args: []string{"--", "-3", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.4: boundary — descending single step.
		{
			Name: "R4.4_boundary_descending_single_step",
			Args: []string{"5", "-5", "1"},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.4: boundary — float sequence last barely reachable.
		{
			Name: "R4.4_boundary_float_last_exact",
			Args: []string{"0.0", "0.1", "0.3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.4: boundary — equal-width with negative start.
		{
			Name: "R4.4_boundary_equal_width_negative",
			Args: []string{"-w", "--", "-1", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffHelpVersion tests --help and --version output.
func TestDiffHelpVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	t.Run("help_exit_0", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(goBin, "--help")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("--help failed: %v", err)
		}
		if len(out) == 0 {
			t.Fatal("--help produced no output")
		}
		if !bytes.Contains(out, []byte("Usage:")) {
			t.Fatalf("--help output missing 'Usage:': %s", out)
		}
	})

	t.Run("version_exit_0", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(goBin, "--version")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("--version failed: %v", err)
		}
		if len(out) == 0 {
			t.Fatal("--version produced no output")
		}
		if !bytes.Contains(out, []byte("seq")) {
			t.Fatalf("--version output missing 'seq': %s", out)
		}
	})
}

// TestDiffErrorCases tests error handling with expected non-zero exit codes.
func TestDiffErrorCases(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gseq")
	if err != nil {
		t.Skipf("reference binary gseq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.5: zero step is an error.
		{
			Name:      "R1.5_zero_step",
			Args:      []string{"1", "0", "5"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R3.2: format with no directive.
		{
			Name:      "R3.2_format_no_directive",
			Args:      []string{"-f", "hello", "1", "3"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R3.2: format with invalid directive.
		{
			Name:      "R3.2_format_invalid_directive",
			Args:      []string{"-f", "%d", "1", "3"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R3.2: format with too many directives.
		{
			Name:      "R3.2_format_too_many_directives",
			Args:      []string{"-f", "%f%f", "1", "3"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R3.2: format with %s (string, not float).
		{
			Name:      "R3.2_format_string_directive",
			Args:      []string{"-f", "%s", "1", "3"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4.2: non-numeric argument is an error.
		{
			Name:      "R4.2_non_numeric_arg",
			Args:      []string{"abc"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4.2: non-numeric FIRST argument with valid LAST.
		{
			Name:      "R4.2_non_numeric_first",
			Args:      []string{"abc", "5"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4.2: missing operand (no arguments).
		{
			Name:      "R4.2_missing_operand",
			Args:      []string{},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4.2: extra operand (four positional arguments).
		{
			Name:      "R4.2_extra_operand",
			Args:      []string{"1", "1", "5", "10"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// normalizeProgramName normalizes error messages for differential comparison.
// GNU seq reports errors as "gseq:" while our binary uses "seq:", and the
// "Try" line includes the full binary path which differs between binaries.
func normalizeProgramName(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gseq"), []byte("seq"))
	// Normalize "Try '/path/to/seq --help'" to "Try 'seq --help'" to handle
	// different binary paths.
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		if bytes.HasPrefix(line, []byte("Try '")) && bytes.HasSuffix(line, []byte("' for more information.")) {
			lines[i] = []byte("Try 'seq --help' for more information.")
		}
	}
	return bytes.Join(lines, []byte("\n"))
}
