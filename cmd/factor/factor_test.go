// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd065-factor R1.1–R1.4, R2.1–R2.4, R3.1–R3.4, R4.1–R4.4.
package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeProgramName replaces "gfactor:" with "factor:" in output
// so differential tests pass despite different binary names.
func normalizeProgramName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gfactor:"), []byte("factor:"))
}

// normalizeHelpOutput discards stdout content for --help/--version tests
// where output text inherently differs between GNU and Go implementations.
// Exit code comparison still applies.
func normalizeHelpOutput(data []byte) []byte {
	return nil
}

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
		// R2.1: stdin mode with single number
		{
			Name:  "stdin_single",
			Stdin: []byte("42\n"),
		},
		// R2.1: stdin mode with multiple numbers
		{
			Name:  "stdin_multiple",
			Stdin: []byte("12\n97\n1\n60\n"),
		},
		// R2.2: max int64 value (2^63-1)
		{
			Name: "max_int64",
			Args: []string{"9223372036854775807"},
		},
		// R2.2: large composite near int64 max
		{
			Name: "large_composite_near_max",
			Args: []string{"9223372036854775806"},
		},
		// R2.3: stdin with blank lines skipped
		{
			Name:  "stdin_blank_lines",
			Stdin: []byte("\n12\n\n97\n\n"),
		},
		// R2.4: non-numeric input produces error
		{
			Name:      "error_non_numeric",
			Args:      []string{"abc"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R2.4: negative number via stdin produces error
		{
			Name:      "error_negative_stdin",
			Stdin:     []byte("-5\n"),
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R2.4: mixed valid and invalid args, continues processing
		{
			Name:      "error_mixed_args",
			Args:      []string{"12", "abc", "97"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R2.4: stdin error handling continues processing
		{
			Name:      "stdin_error_continues",
			Stdin:     []byte("12\nabc\n97\n"),
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R3.1: --help prints usage and exits 0
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{normalizeHelpOutput},
		},
		// R3.2: --version prints version and exits 0
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{normalizeHelpOutput},
		},
		// R3.4: error on invalid input does not stop processing
		{
			Name:      "error_continues_processing",
			Args:      []string{"abc", "12", "xyz", "97"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4.1: exit 0 for single valid composite
		{
			Name: "r4_exit0_single",
			Args: []string{"100"},
		},
		// R4.1: exit 0 for multiple valid inputs
		{
			Name: "r4_exit0_multiple",
			Args: []string{"2", "4", "8", "16", "32"},
		},
		// R4.1: exit 0 for valid stdin input
		{
			Name:  "r4_exit0_stdin",
			Stdin: []byte("50\n100\n"),
		},
		// R4.2: exit 1 for non-numeric argument
		{
			Name:      "r4_exit1_non_numeric_arg",
			Args:      []string{"notanumber"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4.2: exit 1 for empty string via stdin
		{
			Name:      "r4_exit1_stdin_non_numeric",
			Stdin:     []byte("xyz\n"),
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4.2: exit 1 for stdin with invalid input
		{
			Name:      "r4_exit1_stdin_invalid",
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4.4: stdin with multiple valid and invalid lines
		{
			Name:      "r4_stdin_mixed_valid_invalid",
			Stdin:     []byte("12\n-3\n97\nfoo\n1\n"),
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4.4: large prime
		{
			Name: "r4_large_prime",
			Args: []string{"999999999999999877"},
		},
		// R4.4: perfect square
		{
			Name: "r4_perfect_square",
			Args: []string{"49"},
		},
		// R4.4: power of a prime
		{
			Name: "r4_prime_power",
			Args: []string{"243"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
