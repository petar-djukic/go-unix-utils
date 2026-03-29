// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/hostid (prd048 R3.1, R3.2, R3.3).
package main_test

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
// Used for error messages where GNU includes the full binary path,
// causing unavoidable divergence.
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath("ghostid")
	if err != nil {
		t.Skipf("reference binary ghostid not in PATH: %v", err)
	}

	env := []string{"LC_ALL=C"}

	// R3.1: compare stdout and exit codes via RunDiffTests.
	// R3.2: cover normal invocation, extra operand error, unknown flag error.
	// R3.3: all tests set LC_ALL=C.
	tests := []testutils.DiffTest{
		{
			Name: "no arguments",
			Args: []string{},
			Env:  env,
		},
		{
			Name:      "extra operand",
			Args:      []string{"extraarg"},
			Env:       env,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		{
			Name:      "unknown short flag",
			Args:      []string{"-x"},
			Env:       env,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		{
			Name:      "unknown long flag",
			Args:      []string{"--bogus"},
			Env:       env,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
