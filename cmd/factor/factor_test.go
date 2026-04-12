// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/factor.
// Tests cover srd065-factor R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgRe matches the program name/path prefix before a colon at line start.
var stderrProgRe = regexp.MustCompile(`(?m)^[^\s:]+:`)

// stderrTryRe matches the quoted program reference in Try hint lines.
var stderrTryRe = regexp.MustCompile(`'[^']*--help'`)

// stderrNormalizer normalizes program name differences in error messages.
// R3.4/R4.2: replaces binary paths with "PROG" so error message structure
// can be compared between Go and GNU binaries.
func stderrNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	b = stderrProgRe.ReplaceAll(b, []byte("PROG:"))
	b = stderrTryRe.ReplaceAll(b, []byte("'PROG --help'"))
	return b
}

// discardOutput normalizes by discarding all output, used when
// output content differs by design (--version, --help) and only
// exit code comparison is meaningful.
func discardOutput(b []byte) []byte {
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
		// R1.1: composite number factorization format.
		{
			Name: "composite_12",
			Args: []string{"12"},
		},
		{
			Name: "composite_100",
			Args: []string{"100"},
		},

		// R1.2: the number 1 has no factors.
		{
			Name: "one",
			Args: []string{"1"},
		},

		// R1.3: prime numbers print the number as the sole factor.
		{
			Name: "prime_2",
			Args: []string{"2"},
		},
		{
			Name: "prime_97",
			Args: []string{"97"},
		},
		{
			Name: "prime_7919",
			Args: []string{"7919"},
		},

		// R1.4: multiple arguments produce one line each.
		{
			Name: "multiple_args",
			Args: []string{"12", "97", "1", "100"},
		},

		// R2.1: stdin mode reads one integer per line.
		{
			Name: "stdin_single",
			Stdin: []byte("15\n"),
		},
		{
			Name: "stdin_multiple",
			Stdin: []byte("12\n97\n100\n"),
		},

		// R2.2: large number near int64 max.
		{
			Name: "large_number",
			Args: []string{"9223372036854775783"},
		},
		{
			Name: "large_composite",
			Args: []string{"9999999999999"},
		},

		// R2.3: blank lines in stdin are skipped.
		{
			Name: "stdin_blank_lines",
			Stdin: []byte("12\n\n97\n\n"),
		},

		// R2.4/R4.2: non-integer input produces error, exit 1.
		{
			Name:      "error_non_integer",
			Args:      []string{"abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R2.4/R4.2: negative input produces error, exit 1.
		{
			Name:      "error_negative",
			Args:      []string{"-5"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R2.4/R3.4: invalid input mixed with valid, processing continues.
		{
			Name:      "mixed_valid_invalid",
			Args:      []string{"12", "abc", "97"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R2.4: stdin with non-integer produces error, continues.
		{
			Name:      "stdin_error_non_integer",
			Stdin:     []byte("12\nabc\n97\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R3.1: --help prints usage and exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R3.2: --version prints version and exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R4.1: all valid inputs exit 0.
		{
			Name: "exit_0_all_valid",
			Args: []string{"1", "2", "12", "97"},
		},

		// R4.4: power of two (composite with repeated factors).
		{
			Name: "power_of_two",
			Args: []string{"1024"},
		},

		// R4.4: zero is handled.
		{
			Name: "zero",
			Args: []string{"0"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
