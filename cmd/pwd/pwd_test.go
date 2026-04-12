// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/pwd.
// Tests cover srd051-pwd R2.1, R2.2, R3.1, R3.2, R3.3.
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
// R3.2: replaces binary paths with "PROG" so error message structure
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
	refBin, err := exec.LookPath("gpwd")
	if err != nil {
		t.Skipf("reference binary gpwd not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R3.1/R3.2: default invocation prints physical working directory.
		{
			Name: "default_no_args",
			Args: []string{},
		},

		// R3.2: -P flag prints physical path with symlinks resolved.
		{
			Name: "physical_short_flag",
			Args: []string{"-P"},
		},

		// R3.2: --physical long flag prints physical path.
		{
			Name: "physical_long_flag",
			Args: []string{"--physical"},
		},

		// R3.2: -L flag prints logical path from PWD.
		{
			Name: "logical_short_flag",
			Args: []string{"-L"},
		},

		// R3.2: --logical long flag prints logical path.
		{
			Name: "logical_long_flag",
			Args: []string{"--logical"},
		},

		// R3.2: -L -P precedence — last flag wins, physical mode.
		{
			Name: "logical_then_physical",
			Args: []string{"-L", "-P"},
		},

		// R3.2: -P -L precedence — last flag wins, logical mode.
		{
			Name: "physical_then_logical",
			Args: []string{"-P", "-L"},
		},

		// R3.1: --help flag exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R3.1: --version flag exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R2.2: unknown flag produces error and exits non-zero.
		{
			Name:      "unknown_flag",
			Args:      []string{"--bogus"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R3.3: explicit LC_ALL=C environment verifies locale-independent output.
		{
			Name: "explicit_lc_all_c",
			Args: []string{},
			Env:  []string{"LC_ALL=C"},
		},

		// R3.3: explicit LC_ALL=C with -P flag.
		{
			Name: "explicit_lc_all_c_physical",
			Args: []string{"-P"},
			Env:  []string{"LC_ALL=C"},
		},

		// R3.3: explicit LC_ALL=C with -L flag.
		{
			Name: "explicit_lc_all_c_logical",
			Args: []string{"-L"},
			Env:  []string{"LC_ALL=C"},
		},

		// R2.1: extra operand — gpwd ignores non-option arguments with a
		// warning and still prints the working directory (exit 0). Stderr
		// differs in program name so normalize it.
		{
			Name:      "extra_operand",
			Args:      []string{"extraarg"},
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
