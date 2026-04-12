// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/tty.
// Tests cover srd052-tty R2.2, R3.1, R3.2, R3.3.
//
// NOTE (D4): Terminal-connected tests (where stdin must be a real TTY) cannot
// be verified via differential testing because both binaries run with piped
// stdin in the test harness. All tests here exercise piped-stdin scenarios.
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
	refBin, err := exec.LookPath("gtty")
	if err != nil {
		t.Skipf("reference binary gtty not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R3.1: piped stdin (not a tty) prints "not a tty" and exits 1.
		{
			Name:     "piped_stdin_not_a_tty",
			Args:     []string{},
			ExitCode: 1,
		},

		// R3.2: -s flag suppresses output, exits 1 when stdin is piped.
		{
			Name:     "silent_short_flag",
			Args:     []string{"-s"},
			ExitCode: 1,
		},

		// R3.2: --silent flag suppresses output, exits 1 when stdin is piped.
		{
			Name:     "silent_long_flag",
			Args:     []string{"--silent"},
			ExitCode: 1,
		},

		// R3.2: --quiet flag suppresses output, exits 1 when stdin is piped.
		{
			Name:     "quiet_long_flag",
			Args:     []string{"--quiet"},
			ExitCode: 1,
		},

		// R3.3/R2.2: --help flag exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R3.3/R2.2: --version flag exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R3.3/R2.2: unknown flag produces error and exits 2.
		{
			Name:      "unknown_flag",
			Args:      []string{"--bogus"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R3.3/R2.1: extra operand produces error and exits 2.
		{
			Name:      "extra_operand",
			Args:      []string{"extraarg"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
