// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/pwd against gpwd (GNU coreutils).
// Implements prd051-pwd R3.1 (differential tests), R3.2 (coverage),
// R3.3 (LC_ALL=C environment).
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpwd")
	if err != nil {
		t.Skipf("reference binary gpwd not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		{
			Name:     "no_args",
			Args:     nil,
			ExitCode: 0,
		},
		{
			Name:     "physical_flag",
			Args:     []string{"-P"},
			ExitCode: 0,
		},
		{
			Name:     "logical_flag",
			Args:     []string{"-L"},
			ExitCode: 0,
		},
		{
			Name:     "physical_long",
			Args:     []string{"--physical"},
			ExitCode: 0,
		},
		{
			Name:     "logical_long",
			Args:     []string{"--logical"},
			ExitCode: 0,
		},
		{
			// R1.4: last flag wins — -L then -P → physical
			Name:     "logical_then_physical",
			Args:     []string{"-L", "-P"},
			ExitCode: 0,
		},
		{
			// R1.4: last flag wins — -P then -L → logical
			Name:     "physical_then_logical",
			Args:     []string{"-P", "-L"},
			ExitCode: 0,
		},
		{
			Name:     "repeated_physical",
			Args:     []string{"-P", "-P"},
			ExitCode: 0,
		},
		{
			Name:     "repeated_logical",
			Args:     []string{"-L", "-L"},
			ExitCode: 0,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
