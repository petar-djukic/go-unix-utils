// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd046-nproc R2.1, R2.2, R2.3, R3.1, R3.2, R3.3 (differential tests)
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for nproc.
const refBinaryName = "gnproc"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// normalizeProgramName replaces the reference binary path/name with a
	// stable token so stderr messages compare equal despite different argv[0].
	programNamePattern := regexp.MustCompile(`(?:` + regexp.QuoteMeta(refBin) + `|gnproc|nproc)`)
	normalizeProgramName := func(b []byte) []byte {
		return programNamePattern.ReplaceAll(b, []byte("PROG"))
	}

	// normalizeTryPath strips absolute path from "Try '...' --help" messages.
	tryPattern := regexp.MustCompile(`Try '[^']*'`)
	normalizeTryPath := func(b []byte) []byte {
		return tryPattern.ReplaceAll(b, []byte("Try 'PROG'"))
	}

	stderrNorm := []testutils.NormalizeFunc{normalizeProgramName, normalizeTryPath}

	tests := []testutils.DiffTest{
		// R3.2: default invocation — prints CPU count, exit 0.
		{
			Name: "no_args",
			Args: []string{},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: --all flag — prints installed processor count, exit 0.
		{
			Name: "all_flag",
			Args: []string{"--all"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: --ignore=1 — prints count minus 1, exit 0.
		{
			Name: "ignore_1",
			Args: []string{"--ignore=1"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: --all --ignore=1 — combined, exit 0.
		{
			Name: "all_ignore_1",
			Args: []string{"--all", "--ignore=1"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: extra operand — exit 1 with error on stderr.
		{
			Name:      "extra_operand",
			Args:      []string{"extraarg"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// R2.2: --ignore with non-numeric value — exit 1 with error on stderr.
		{
			Name:      "ignore_non_numeric",
			Args:      []string{"--ignore=abc"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// R2.2: --ignore (space-separated) with non-numeric value — exit 1.
		{
			Name:      "ignore_space_non_numeric",
			Args:      []string{"--ignore", "abc"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// R2.3: unknown flag — exit 1 with error on stderr.
		{
			Name:      "unknown_flag",
			Args:      []string{"--unknown"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffHelpVersion verifies --help and --version exit 0.
// Output content differs between implementations, so stdout/stderr are
// normalized to empty; only exit codes are compared.
func TestDiffHelpVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	clearOutput := func(b []byte) []byte { return nil }

	tests := []testutils.DiffTest{
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
