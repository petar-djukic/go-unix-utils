// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/factor against gfactor (GNU coreutils).
//
// Covers prd065-factor R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R3.4.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeBinaryName replaces program name prefixes in error messages
// so that "gfactor:" and "factor:" both become "PROG:".
var normalizeBinaryName testutils.NormalizeFunc = func(data []byte) []byte {
	return binaryNameRe.ReplaceAll(data, []byte("PROG:"))
}

var binaryNameRe = regexp.MustCompile(`(?m)^g?factor:`)

// normalizeHelpVersion strips all output content for --help and --version
// tests, keeping only the exit code comparison. GNU and Go binaries produce
// different help/version text, so byte-level comparison is not meaningful.
var normalizeHelpVersion testutils.NormalizeFunc = func(data []byte) []byte {
	return nil
}

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
		// R1.3, R2.2: large prime within int64 range
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
		// R2.1: stdin mode with single number
		{
			Name:     "R2.1_stdin_single",
			Stdin:    []byte("15\n"),
			ExitCode: 0,
		},
		// R2.1: stdin mode with multiple lines
		{
			Name:     "R2.1_stdin_multiple",
			Stdin:    []byte("12\n97\n15\n"),
			ExitCode: 0,
		},
		// R2.2: large composite within int64 range (2^40)
		{
			Name:     "R2.2_large_power_of_two",
			Args:     []string{"1099511627776"},
			ExitCode: 0,
		},
		// R2.3: blank lines in stdin are skipped
		{
			Name:     "R2.3_blank_lines",
			Stdin:    []byte("12\n\n15\n\n"),
			ExitCode: 0,
		},
		// R2.3: stdin with leading blank line
		{
			Name:     "R2.3_leading_blank",
			Stdin:    []byte("\n12\n"),
			ExitCode: 0,
		},
		// R2.4: non-integer input in stdin
		{
			Name:      "R2.4_non_integer_stdin",
			Stdin:     []byte("abc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// R2.4: negative number in stdin
		{
			Name:      "R2.4_negative_stdin",
			Stdin:     []byte("-5\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// R2.4: mixed valid and invalid in stdin — continues processing
		{
			Name:      "R2.4_mixed_valid_invalid",
			Stdin:     []byte("12\nabc\n15\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// R3.1: --help prints usage to stdout and exits 0
		{
			Name:      "R3.1_help",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeHelpVersion},
		},
		// R3.2: --version prints version info to stdout and exits 0
		{
			Name:      "R3.2_version",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeHelpVersion},
		},
		// R3.3: factorization output goes to stdout (verified by all passing tests)
		{
			Name:     "R3.3_output_stdout",
			Args:     []string{"42"},
			ExitCode: 0,
		},
		// R3.4: error to stderr without stopping — mixed valid and invalid args
		{
			Name:      "R3.4_error_continues",
			Args:      []string{"12", "notanumber", "15"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
