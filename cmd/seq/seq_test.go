// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/seq.
// Tests cover srd019-seq R1.1-R1.4.
package main

import (
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
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
