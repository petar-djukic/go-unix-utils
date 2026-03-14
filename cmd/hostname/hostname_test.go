// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd047-hostname R3.1, R3.2, R3.3 (differential tests)
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for hostname.
const refBinaryName = "ghostname"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// normalizeProgramName replaces the reference binary path/name with the
	// Go binary name so stderr messages compare equal despite different argv[0].
	programNamePattern := regexp.MustCompile(`(?:` + regexp.QuoteMeta(refBin) + `|ghostname|hostname)`)
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
		// R3.2: normal invocation — prints hostname, exit 0.
		{
			Name: "no_args",
			Args: []string{},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: extra operand — exit 1 with error on stderr.
		{
			Name:      "extra_operand",
			Args:      []string{"extraarg"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// R3.2: unknown flag — exit 1 with error on stderr.
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
