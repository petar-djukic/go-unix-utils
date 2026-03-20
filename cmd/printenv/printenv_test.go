// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd040-printenv R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// clearOutput returns a normalizer that replaces all output with a fixed
// marker so only exit codes are compared. Used for --version and --help
// where output text differs between Go and GNU binaries.
func clearOutput(b []byte) []byte {
	if len(b) > 0 {
		return []byte("OUTPUT_PRESENT\n")
	}
	return b
}

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
			// R1.2, R1.3, R2.3: mix of existing and missing, exit 1
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
			// R1.3, R3.3: no stderr for missing variable
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
		{
			// R2.2: exit 0 when all requested variables are found
			Name: "exit_zero_all_found",
			Args: []string{"HOME", "PATH"},
		},
		{
			// R2.4: no arguments always exits 0
			Name: "no_args_exit_zero",
			Args: []string{},
		},
		{
			// R2.2, R3.1: --null with multiple variables, each NUL-terminated
			Name: "null_multiple_variables",
			Args: []string{"--null", "HOME", "PATH"},
		},
		{
			// R2.2, R3.1: -0 short flag with multiple variables
			Name: "null_short_multiple_variables",
			Args: []string{"-0", "HOME", "PATH"},
		},
		{
			// R3.2: no-argument full dump with NUL-delimited output
			Name: "null_no_args_full_dump",
			Args: []string{"-0"},
		},
		{
			// R3.2, R3.3: multiple missing variables, exit 1, no stderr
			Name:     "multiple_missing_variables",
			Args:     []string{"PRINTENV_TEST_MISSING_A", "PRINTENV_TEST_MISSING_B"},
			ExitCode: 1,
		},
		{
			// R3.2, R3.3: mix of existing and missing with NUL termination
			Name:     "null_mix_existing_and_missing",
			Args:     []string{"-0", "HOME", "PRINTENV_TEST_NONEXISTENT_VAR_XYZ"},
			ExitCode: 1,
		},
		{
			// R3.3: no stderr for mix of existing and missing variables
			Name:     "no_stderr_mix_existing_missing",
			Args:     []string{"HOME", "PATH", "PRINTENV_TEST_NONEXISTENT_VAR_XYZ"},
			ExitCode: 1,
		},
		{
			// R2.3: --version prints version info and exits 0
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			// R2.4: --help prints usage and exits 0
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
