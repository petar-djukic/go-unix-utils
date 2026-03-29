// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/printenv against gprintenv (GNU coreutils).
//
// Covers prd040-printenv R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R2.4, R3.1.
package main

import (
	"bytes"
	"os/exec"
	"sort"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// sortLines sorts output lines so full-dump comparison is order-independent.
func sortLines(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	sort.Slice(lines, func(i, j int) bool {
		return bytes.Compare(lines[i], lines[j]) < 0
	})
	return bytes.Join(lines, []byte("\n"))
}

// sortNulLines sorts NUL-delimited entries for order-independent comparison.
func sortNulLines(data []byte) []byte {
	entries := bytes.Split(data, []byte{0})
	sort.Slice(entries, func(i, j int) bool {
		return bytes.Compare(entries[i], entries[j]) < 0
	})
	return bytes.Join(entries, []byte{0})
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gprintenv")
	if err != nil {
		t.Skip("reference binary gprintenv not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1: no arguments prints all environment variables
		{
			Name:      "R1.1_print_all",
			Args:      []string{},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R1.2: single existing variable prints only its value
		{
			Name:     "R1.2_single_variable",
			Args:     []string{"HOME"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.2: multiple existing variables print values in order
		{
			Name:     "R1.2_multiple_variables",
			Args:     []string{"HOME", "PATH"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.3: missing variable produces no output, exit 1
		{
			Name:     "R1.3_missing_variable",
			Args:     []string{"DOES_NOT_EXIST_XYZZY"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R1.3: mix of existing and missing variables, exit 1
		{
			Name:     "R1.3_mixed_existing_missing",
			Args:     []string{"HOME", "DOES_NOT_EXIST_XYZZY"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R2.1: -0 flag uses NUL terminator
		{
			Name:     "R2.1_null_delimited",
			Args:     []string{"-0", "HOME"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1: --null long flag uses NUL terminator
		{
			Name:     "R2.1_null_long_flag",
			Args:     []string{"--null", "HOME"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1: -0 with no arguments (full dump, NUL-delimited)
		{
			Name:      "R2.1_null_all",
			Args:      []string{"-0"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{sortNulLines},
		},
		// R2.2: exit 0 when all requested variables exist
		{
			Name:     "R2.2_exit_0_all_found",
			Args:     []string{"HOME", "PATH"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.3: exit 1 when any requested variable is missing
		{
			Name:     "R2.3_exit_1_any_missing",
			Args:     []string{"HOME", "PRINTENV_NONEXISTENT_VAR_42"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R2.3: exit 1 when all requested variables are missing
		{
			Name:     "R2.3_exit_1_all_missing",
			Args:     []string{"PRINTENV_NONEXISTENT_A", "PRINTENV_NONEXISTENT_B"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R2.4: no arguments always exits 0
		{
			Name:      "R2.4_no_args_exit_0",
			Args:      []string{},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R3.1: differential test with -0 and mixed existing/missing
		{
			Name:     "R3.1_null_mixed_existing_missing",
			Args:     []string{"-0", "HOME", "PRINTENV_NONEXISTENT_VAR_99"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
