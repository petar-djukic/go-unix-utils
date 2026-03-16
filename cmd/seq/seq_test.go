// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/seq against gseq (GNU coreutils).
// Implements prd019-seq R1.1-R1.4 test coverage.
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
