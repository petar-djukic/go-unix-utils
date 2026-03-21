// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd065-factor R1.1–R1.4.
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
		t.Skipf("reference binary gfactor not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: composite number factorization
		{
			Name: "composite_12",
			Args: []string{"12"},
		},
		// R1.1: another composite
		{
			Name: "composite_60",
			Args: []string{"60"},
		},
		// R1.2: factor of 1 prints no factors
		{
			Name: "one",
			Args: []string{"1"},
		},
		// R1.2: factor of 0
		{
			Name: "zero",
			Args: []string{"0"},
		},
		// R1.3: prime number
		{
			Name: "prime_97",
			Args: []string{"97"},
		},
		// R1.3: small prime
		{
			Name: "prime_2",
			Args: []string{"2"},
		},
		// R1.3: another prime
		{
			Name: "prime_13",
			Args: []string{"13"},
		},
		// R1.1: power of two
		{
			Name: "power_of_two",
			Args: []string{"64"},
		},
		// R1.4: multiple arguments
		{
			Name: "multiple_args",
			Args: []string{"12", "97", "1", "60"},
		},
		// R1.4: multiple primes
		{
			Name: "multiple_primes",
			Args: []string{"2", "3", "5", "7", "11"},
		},
		// R1.1: large composite
		{
			Name: "large_composite",
			Args: []string{"9999999999999"},
		},
		// R1.1: large number near int64 max
		{
			Name: "large_number",
			Args: []string{"999999999999989"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
