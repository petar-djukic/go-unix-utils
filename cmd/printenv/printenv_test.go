// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd040-printenv R1.1, R1.2, R1.3, R2.1.
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
		{
			// R1.2: single existing variable prints its value
			Name: "single_existing_variable",
			Args: []string{"HOME"},
		},
		{
			// R1.2: multiple existing variables print values in order
			Name: "multiple_existing_variables",
			Args: []string{"HOME", "PATH"},
		},
		{
			// R1.3, R2.3: missing variable produces no output, exit 1
			Name:     "missing_variable",
			Args:     []string{"PRINTENV_TEST_NONEXISTENT_VAR_XYZ"},
			ExitCode: 1,
		},
		{
			// R1.2, R1.3: mix of existing and missing, exit 1
			Name:     "mix_existing_and_missing",
			Args:     []string{"HOME", "PRINTENV_TEST_NONEXISTENT_VAR_XYZ"},
			ExitCode: 1,
		},
		{
			// R2.1: NUL-delimited output for single variable
			Name: "null_delimiter_single",
			Args: []string{"-0", "HOME"},
		},
		{
			// R2.1: NUL-delimited with --null long option
			Name: "null_delimiter_long_option",
			Args: []string{"--null", "HOME"},
		},
		{
			// R1.3: no stderr for missing variable
			Name:     "no_stderr_for_missing",
			Args:     []string{"PRINTENV_TEST_NONEXISTENT_VAR_XYZ"},
			ExitCode: 1,
		},
		{
			// R1.2: variable with empty value
			Name: "empty_value_variable",
			Args: []string{"PRINTENV_TEST_EMPTY_VAR"},
			Env:  []string{"PRINTENV_TEST_EMPTY_VAR="},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
