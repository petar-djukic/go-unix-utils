// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd065-factor R1.1–R1.4, R2.1–R2.4.
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
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
