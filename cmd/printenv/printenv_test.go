// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/printenv against the GNU reference binary (gprintenv).
//
// Implements prd040-printenv acceptance criteria AC1-AC6 via testutils.RunDiffTests.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gprintenv")
	if err != nil {
		t.Skipf("reference binary gprintenv not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: No arguments prints all environment variables.
		{
			Name: "printenv_no_args",
			Args: []string{},
		},
		// R1.2: Print value of a single existing variable.
		{
			Name: "printenv_single_existing",
			Args: []string{"HOME"},
		},
		// R1.2: Print values of multiple existing variables.
		{
			Name: "printenv_multiple_existing",
			Args: []string{"HOME", "USER"},
		},
		// R2.3: Exit 1 when variable is not set.
		{
			Name: "printenv_missing_variable",
			Args:     []string{"SURELY_NONEXISTENT_VAR_XYZ"},
			ExitCode: 1,
		},
		// R2.3: Mix of existing and missing variables, exit 1.
		{
			Name: "printenv_mixed_existing_missing",
			Args:     []string{"HOME", "SURELY_NONEXISTENT_VAR_XYZ"},
			ExitCode: 1,
		},
		// R2.1: -0 terminates output with NUL.
		{
			Name: "printenv_null_terminator",
			Args: []string{"-0", "HOME"},
		},
		// R2.1: -0 with no arguments.
		{
			Name: "printenv_null_no_args",
			Args: []string{"-0"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
