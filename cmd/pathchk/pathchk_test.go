// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/pathchk.
// Tests cover srd103-pathchk R1.1-R1.4, R2.1-R2.3.
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
func stderrNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	b = stderrProgRe.ReplaceAll(b, []byte("PROG:"))
	b = stderrTryRe.ReplaceAll(b, []byte("'PROG --help'"))
	return b
}

// discardOutput normalizes by discarding all output, used when
// output content differs by design (--version, --help).
func discardOutput(b []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpathchk")
	if err != nil {
		t.Skipf("reference binary gpathchk not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: valid path exits 0.
		{
			Name: "valid_simple",
			Args: []string{"validpath"},
		},
		{
			Name: "valid_nested",
			Args: []string{"/usr/bin/sort"},
		},
		{
			Name: "valid_relative",
			Args: []string{"a/b/c"},
		},

		// R1.1: empty path in default mode.
		{
			Name:      "empty_default",
			Args:      []string{""},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R1.2: -p portable character set checks.
		{
			Name: "posix_valid",
			Args: []string{"-p", "valid_path.txt"},
		},
		{
			Name:      "posix_invalid_char",
			Args:      []string{"-p", "invalid@path"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		{
			Name:      "posix_space_in_name",
			Args:      []string{"-p", "has space"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		{
			Name:      "posix_long_component",
			Args:      []string{"-p", "abcdefghijklmno"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		{
			Name: "posix_max_component",
			Args: []string{"-p", "abcdefghijklmn"},
		},

		// R1.3: -P extra portability checks (use -- to separate from operands).
		{
			Name:      "extra_leading_dash",
			Args:      []string{"-P", "--", "-filename"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		{
			Name:      "extra_empty",
			Args:      []string{"-P", ""},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		{
			Name: "extra_valid",
			Args: []string{"-P", "validname"},
		},
		{
			Name:      "extra_leading_dash_in_component",
			Args:      []string{"-P", "dir/-file"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R1.3 + R1.2: combined -p -P (--portability).
		{
			Name:      "portability_flag",
			Args:      []string{"--portability", "--", "-leadingdash"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		{
			Name:      "combined_pP_invalid_char",
			Args:      []string{"-pP", "bad@name"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R1.4: multiple pathnames.
		{
			Name: "multi_valid",
			Args: []string{"a", "b", "c"},
		},
		{
			Name:      "multi_one_invalid",
			Args:      []string{"-p", "valid", "invalid@"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R1.4: --help and --version.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// No operand error.
		{
			Name:      "no_args",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
