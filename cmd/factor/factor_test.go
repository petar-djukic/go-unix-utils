// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/factor against GNU gfactor.
// Covers prd065-factor R4.1-R4.4 (exit codes and differential testing).
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes error messages between GNU gfactor and Go factor.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?factor|gfactor`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	// GNU treats "-N" as invalid option; Go treats it as invalid integer.
	negativeOpt := regexp.MustCompile(
		`(?:invalid option -- '[^']*'|'[^']*' is not a valid positive integer)`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("factor"))
		b = tryHelp.ReplaceAll(b, nil)
		b = negativeOpt.ReplaceAll(b, []byte("invalid input"))
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfactor")
	if err != nil {
		t.Skipf("reference binary gfactor not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		// R4.1/R4.4: prime number — single factor.
		{
			Name: "prime_97",
			Args: []string{"97"},
		},
		// R4.1/R4.4: composite number — multiple factors.
		{
			Name: "composite_12",
			Args: []string{"12"},
		},
		// R4.1/R4.4: number 1 — no factors.
		{
			Name: "one",
			Args: []string{"1"},
		},
		// R4.1/R4.4: power of 2.
		{
			Name: "power_of_2_64",
			Args: []string{"64"},
		},
		// R4.1/R4.4: large prime.
		{
			Name: "large_prime_999999937",
			Args: []string{"999999937"},
		},
		// R4.1/R4.4: small composite with repeated factor.
		{
			Name: "repeated_factor_8",
			Args: []string{"8"},
		},
		// R4.1/R4.4: number 2 — smallest prime.
		{
			Name: "prime_2",
			Args: []string{"2"},
		},
		// R4.3/R4.4: large number near 2^63.
		{
			Name: "large_number_2pow62",
			Args: []string{"4611686018427387904"},
		},
		// R4.3/R4.4: large semiprime.
		{
			Name: "large_semiprime",
			Args: []string{"9999999999999937"},
		},
		// R4.4: multiple arguments in order.
		{
			Name: "multiple_args",
			Args: []string{"6", "7", "8", "9", "10"},
		},
		// R4.4: stdin mode — single number.
		{
			Name: "stdin_single",
			Args: []string{},
			Stdin: []byte("42\n"),
		},
		// R4.4: stdin mode — multiple numbers.
		{
			Name: "stdin_multiple",
			Args: []string{},
			Stdin: []byte("15\n28\n1\n"),
		},
		// R4.2/R4.4: non-numeric input via argument.
		{
			Name:      "error_non_numeric",
			Args:      []string{"abc"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.2/R4.4: negative number via argument.
		{
			Name:      "error_negative",
			Args:      []string{"-1"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.2/R4.4: mixed valid and invalid arguments.
		{
			Name:      "mixed_valid_invalid",
			Args:      []string{"12", "abc", "7"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.1/R4.4: number 0.
		{
			Name: "zero",
			Args: []string{"0"},
		},
		// R4.4: stdin mode with blank lines (should be skipped).
		{
			Name:  "stdin_blank_lines",
			Args:  []string{},
			Stdin: []byte("6\n\n10\n"),
		},
		// R4.1/R4.4: large power of 2.
		{
			Name: "power_of_2_large",
			Args: []string{"1073741824"},
		},
		// R4.2/R4.4: floating point input.
		{
			Name:      "error_float",
			Args:      []string{"3.14"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.1/R4.4: product of distinct primes.
		{
			Name: "distinct_primes_2310",
			Args: []string{"2310"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
