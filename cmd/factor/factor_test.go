// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/factor against gfactor (GNU coreutils).
//
// Covers prd065-factor R1.1, R1.2, R1.3, R1.4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfactor")
	if err != nil {
		t.Skip("reference binary gfactor not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1: composite number — ascending factors with multiplicity
		{
			Name:     "R1.1_composite_12",
			Args:     []string{"12"},
			ExitCode: 0,
		},
		// R1.1: larger composite with repeated factors
		{
			Name:     "R1.1_composite_360",
			Args:     []string{"360"},
			ExitCode: 0,
		},
		// R1.1: power of two
		{
			Name:     "R1.1_power_of_two",
			Args:     []string{"64"},
			ExitCode: 0,
		},
		// R1.2: the number 1 prints no factors
		{
			Name:     "R1.2_one",
			Args:     []string{"1"},
			ExitCode: 0,
		},
		// R1.2: zero
		{
			Name:     "R1.2_zero",
			Args:     []string{"0"},
			ExitCode: 0,
		},
		// R1.3: prime number is its own sole factor
		{
			Name:     "R1.3_prime_97",
			Args:     []string{"97"},
			ExitCode: 0,
		},
		// R1.3: small prime
		{
			Name:     "R1.3_prime_2",
			Args:     []string{"2"},
			ExitCode: 0,
		},
		// R1.3: prime 3
		{
			Name:     "R1.3_prime_3",
			Args:     []string{"3"},
			ExitCode: 0,
		},
		// R1.3: large prime
		{
			Name:     "R1.3_large_prime",
			Args:     []string{"999999937"},
			ExitCode: 0,
		},
		// R1.4: multiple arguments processed in order
		{
			Name:     "R1.4_multiple_args",
			Args:     []string{"12", "97", "1", "360"},
			ExitCode: 0,
		},
		// R1.4: two arguments
		{
			Name:     "R1.4_two_args",
			Args:     []string{"15", "28"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
