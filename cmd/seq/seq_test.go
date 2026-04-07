// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/seq.
// Tests cover srd019-seq R1.1-R1.5, R2.1-R2.4, R3.1-R3.4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// dropStderr discards stderr entirely for tests where error messages differ.
func dropStderr(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gseq")
	if err != nil {
		t.Skipf("reference binary gseq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: one-argument form (seq LAST).
		{Name: "one_arg_5", Args: []string{"5"}},
		{Name: "one_arg_1", Args: []string{"1"}},
		{Name: "one_arg_10", Args: []string{"10"}},
		// R1.1: LAST < 1 produces no output.
		{Name: "one_arg_0", Args: []string{"0"}},
		{Name: "one_arg_negative", Args: []string{"-1"}},

		// R1.2: two-argument form (seq FIRST LAST).
		{Name: "two_arg_2_5", Args: []string{"2", "5"}},
		{Name: "two_arg_neg3_3", Args: []string{"-3", "3"}},

		// R1.3: three-argument form (seq FIRST STEP LAST).
		{Name: "three_arg_1_2_10", Args: []string{"1", "2", "10"}},
		{Name: "three_arg_1_3_10", Args: []string{"1", "3", "10"}},
		{Name: "three_arg_descend", Args: []string{"5", "-1", "1"}},
		{Name: "three_arg_descend_2", Args: []string{"10", "-3", "1"}},

		// R1.3: FIRST equals LAST prints one number.
		{Name: "first_eq_last_one", Args: []string{"3", "3"}},
		{Name: "first_eq_last_three", Args: []string{"3", "1", "3"}},

		// R1.4: empty sequences (positive step, FIRST > LAST).
		{Name: "empty_pos_step", Args: []string{"5", "1", "3"}},
		// R1.4: empty sequences (negative step, FIRST < LAST).
		{Name: "empty_neg_step", Args: []string{"1", "-1", "5"}},

		// R1.4: floating-point arguments.
		{Name: "float_half_step", Args: []string{"0.5", "0.5", "2.5"}},
		{Name: "float_first_last", Args: []string{"1.5", "3.5"}},
		{Name: "float_quarter", Args: []string{"0.25", "0.25", "1.0"}},

		// R1.5: descending sequences.
		{Name: "descend_neg_incr", Args: []string{"5", "-1", "1"}},
		{Name: "descend_neg_incr_float", Args: []string{"2.0", "-0.5", "0.0"}},
		{Name: "descend_large_step", Args: []string{"100", "-25", "0"}},

		// R2.2: custom separator (-s).
		{Name: "sep_comma", Args: []string{"-s", ", ", "3"}},
		{Name: "sep_colon", Args: []string{"-s", ":", "1", "4"}},
		{Name: "sep_empty", Args: []string{"-s", "", "3"}},
		{Name: "sep_long_flag", Args: []string{"--separator=, ", "3"}},
		{Name: "sep_tab", Args: []string{"-s", "\t", "1", "3"}},

		// R3.1: format string (-f).
		{Name: "fmt_2f", Args: []string{"-f", "%.2f", "1", "3"}},
		{Name: "fmt_05_2f", Args: []string{"-f", "%05.2f", "1", "3"}},
		{Name: "fmt_e", Args: []string{"-f", "%e", "1", "3"}},
		{Name: "fmt_g", Args: []string{"-f", "%g", "1", "5"}},
		{Name: "fmt_long_flag", Args: []string{"--format=%.1f", "1", "3"}},
		{Name: "fmt_prefix_suffix", Args: []string{"-f", "num:%g:", "1", "3"}},

		// R3.2: invalid format strings exit 1.
		// Stderr wording differs between gseq and seq; drop stderr for comparison.
		{Name: "fmt_invalid_d", Args: []string{"-f", "%d", "1", "3"},
			ExitCode: 1, Normalize: []testutils.NormalizeFunc{dropStderr}},
		{Name: "fmt_invalid_two", Args: []string{"-f", "%f %f", "1", "3"},
			ExitCode: 1, Normalize: []testutils.NormalizeFunc{dropStderr}},
		{Name: "fmt_no_conv", Args: []string{"-f", "hello", "1", "3"},
			ExitCode: 1, Normalize: []testutils.NormalizeFunc{dropStderr}},
		{Name: "fmt_invalid_s", Args: []string{"-f", "%s", "1", "3"},
			ExitCode: 1, Normalize: []testutils.NormalizeFunc{dropStderr}},

		// R3.3: equal-width (-w).
		{Name: "ew_8_12", Args: []string{"-w", "8", "12"}},
		{Name: "ew_1_10", Args: []string{"-w", "1", "10"}},
		{Name: "ew_1_100", Args: []string{"-w", "1", "100"}},
		{Name: "ew_neg5_5", Args: []string{"-w", "-5", "5"}},
		{Name: "ew_long_flag", Args: []string{"--equal-width", "8", "12"}},

		// R3.4: -f and -w together is an error.
		{Name: "fmt_and_ew_error", Args: []string{"-w", "-f", "%.2f", "1", "5"},
			ExitCode: 1, Normalize: []testutils.NormalizeFunc{dropStderr}},

		// R2.4: large integers.
		{Name: "large_int", Args: []string{"9999999999", "1", "9999999999"}},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestZeroStep verifies that zero increment exits non-zero.
// R1.5: STEP must not be zero.
func TestZeroStep(t *testing.T) {
	t.Parallel()
	bin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(bin, "1", "0", "5")
	if err := cmd.Run(); err == nil {
		t.Fatal("expected non-zero exit for zero step")
	}
}

// TestVersion verifies --version prints output and exits 0.
func TestVersion(t *testing.T) {
	t.Parallel()
	bin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(bin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("--version produced no output")
	}
}

// TestHelp verifies --help prints output and exits 0.
func TestHelp(t *testing.T) {
	t.Parallel()
	bin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(bin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("--help produced no output")
	}
}
