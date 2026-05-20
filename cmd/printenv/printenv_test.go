// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main_test

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath("gprintenv")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		{
			Name: "no args full dump",
		},
		{
			Name: "single existing variable",
			Args: []string{"HOME"},
		},
		{
			Name: "multiple existing variables",
			Args: []string{"HOME", "PATH"},
		},
		{
			Name:     "missing variable",
			Args:     []string{"NONEXISTENT_VAR_FOR_PRINTENV_TEST"},
			ExitCode: 1,
		},
		{
			Name:     "mix of existing and missing variables",
			Args:     []string{"HOME", "NONEXISTENT_VAR_FOR_PRINTENV_TEST"},
			ExitCode: 1,
		},
		{
			Name: "null terminated no args",
			Args: []string{"-0"},
		},
		{
			Name: "null terminated single variable",
			Args: []string{"-0", "HOME"},
		},
		{
			Name:     "null terminated missing variable",
			Args:     []string{"-0", "NONEXISTENT_VAR_FOR_PRINTENV_TEST"},
			ExitCode: 1,
		},
		{
			Name: "long flag null",
			Args: []string{"--null", "HOME"},
		},
		{
			Name:     "multiple missing variables",
			Args:     []string{"NONEXISTENT_VAR_FOR_PRINTENV_TEST", "ANOTHER_NONEXISTENT_VAR_TEST"},
			ExitCode: 1,
		},
		{
			Name: "null terminated mix of existing and missing",
			Args: []string{"-0", "HOME", "NONEXISTENT_VAR_FOR_PRINTENV_TEST"},
			ExitCode: 1,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
