// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/nproc.
// Tests cover srd046-nproc R3.1, R3.2, R3.3.
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
// R3.3: replaces binary paths with "PROG" so error message structure
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
	refBin, err := exec.LookPath("gnproc")
	if err != nil {
		t.Skipf("reference binary gnproc not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R3.1/R3.2: default invocation with no arguments.
		{
			Name: "default_no_args",
			Args: []string{},
		},

		// R3.2: --all flag prints installed processor count.
		{
			Name: "flag_all",
			Args: []string{"--all"},
		},

		// R3.2: --ignore=0 should not change the count.
		{
			Name: "ignore_zero",
			Args: []string{"--ignore=0"},
		},

		// R3.2: --ignore=1 subtracts one from available count.
		{
			Name: "ignore_one",
			Args: []string{"--ignore=1"},
		},

		// R3.2: --all --ignore=1 combined.
		{
			Name: "all_ignore_one",
			Args: []string{"--all", "--ignore=1"},
		},

		// R3.2: --ignore with value exceeding CPU count clamps to 1.
		{
			Name: "ignore_exceeds_cpus",
			Args: []string{"--ignore=99999"},
		},

		// R3.2: --all with large --ignore clamps to 1.
		{
			Name: "all_ignore_exceeds_cpus",
			Args: []string{"--all", "--ignore=99999"},
		},

		// R3.2: --help flag exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R3.2: --version flag exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R3.3: unrecognized long flag produces error and exit 1.
		{
			Name:      "unknown_long_flag",
			Args:      []string{"--bogus"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R3.3: invalid --ignore value (non-numeric) produces error and exit 1.
		{
			Name:      "ignore_non_numeric",
			Args:      []string{"--ignore=abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
