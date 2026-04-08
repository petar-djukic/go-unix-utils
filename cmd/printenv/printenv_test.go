// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/printenv.
// Tests cover srd040-printenv R3.1, R3.2, R3.3.
package main

import (
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgRe matches the program name/path prefix before a colon at line start.
var stderrProgRe = regexp.MustCompile(`(?m)^[^\s:]+:`)

// stderrTryRe matches the quoted program reference in Try hint lines.
var stderrTryRe = regexp.MustCompile(`'[^']*--help'`)

// stderrNormalizer normalizes program name differences in error messages.
func stderrNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	b = stderrProgRe.ReplaceAll(b, []byte("PROG:"))
	b = stderrTryRe.ReplaceAll(b, []byte("'PROG --help'"))
	return b
}

// sortLinesNormalizer sorts output lines to handle environment variable
// ordering differences between Go and GNU implementations.
func sortLinesNormalizer(b []byte) []byte {
	s := strings.TrimSuffix(string(b), "\n")
	if s == "" {
		return b
	}
	lines := strings.Split(s, "\n")
	sort.Strings(lines)
	return []byte(strings.Join(lines, "\n") + "\n")
}

// discardOutput normalizes by discarding all output, used when output
// content differs by design (--version, --help) and only exit code
// comparison is meaningful.
func discardOutput(b []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gprintenv")
	if err != nil {
		t.Skipf("reference binary gprintenv not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R3.1: no arguments prints all environment variables.
		{
			Name:      "no_args_explicit_env",
			Args:      []string{},
			Env:       []string{"FOO=bar", "BAZ=qux"},
			Normalize: []testutils.NormalizeFunc{sortLinesNormalizer},
		},

		// R3.1: single existing variable prints its value.
		{
			Name: "single_existing_var",
			Args: []string{"MY_TEST_VAR"},
			Env:  []string{"MY_TEST_VAR=hello"},
		},

		// R3.1: multiple existing variables print values in order.
		{
			Name: "multiple_existing_vars",
			Args: []string{"A", "B", "C"},
			Env:  []string{"A=1", "B=2", "C=3"},
		},

		// R3.1, R3.3: missing variable exits 1, no stderr output.
		{
			Name:     "missing_var",
			Args:     []string{"DOES_NOT_EXIST_XYZZY"},
			Env:      []string{"OTHER=val"},
			ExitCode: 1,
		},

		// R3.1, R3.3: mix of existing and missing variables exits 1.
		{
			Name:     "mix_existing_and_missing",
			Args:     []string{"A", "MISSING", "B"},
			Env:      []string{"A=1", "B=2"},
			ExitCode: 1,
		},

		// R3.2: -0 flag terminates output with NUL instead of newline.
		{
			Name: "null_short_flag",
			Args: []string{"-0", "MY_VAR"},
			Env:  []string{"MY_VAR=value"},
		},

		// R3.2: --null long form of -0.
		{
			Name: "null_long_flag",
			Args: []string{"--null", "MY_VAR"},
			Env:  []string{"MY_VAR=value"},
		},

		// R3.2: -0 with no arguments (full dump with NUL terminators).
		{
			Name: "null_no_args",
			Args: []string{"-0"},
			Env:  []string{"X=1", "Y=2"},
			Normalize: []testutils.NormalizeFunc{func(b []byte) []byte {
				// Sort NUL-delimited entries for ordering independence.
				s := strings.TrimSuffix(string(b), "\x00")
				if s == "" {
					return b
				}
				parts := strings.Split(s, "\x00")
				sort.Strings(parts)
				return []byte(strings.Join(parts, "\x00") + "\x00")
			}},
		},

		// R3.2: --version exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R3.2: --help exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R3.3: invalid option produces error on stderr.
		{
			Name:      "invalid_option",
			Args:      []string{"--bogus"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R3.3: unset variable exits 1, no stderr.
		{
			Name:     "unset_var_no_stderr",
			Args:     []string{"UNSET_VAR_ABC123"},
			Env:      []string{"OTHER=x"},
			ExitCode: 1,
		},

		// R3.1: all requested variables found exits 0.
		{
			Name: "all_found_exit_zero",
			Args: []string{"A", "B"},
			Env:  []string{"A=1", "B=2"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
