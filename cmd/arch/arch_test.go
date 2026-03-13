// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd045-arch R3.1, R3.2, R3.3 (differential tests)
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for arch.
const refBinaryName = "garch"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R3.2: normal invocation — prints machine hardware name, exit 0.
		{
			Name: "no_args",
			Args: []string{},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: extra operand — exit 1 with error on stderr.
		{
			Name:     "extra_operand",
			Args:     []string{"extraarg"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R3.2: unknown flag — exit 1 with error on stderr.
		{
			Name:     "unknown_flag",
			Args:     []string{"--unknown"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
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
