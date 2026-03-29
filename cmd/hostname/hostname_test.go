// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/hostname (prd047 R3.1, R3.2, R3.3).
package main_test

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath("ghostname")
	if err != nil {
		t.Skipf("reference binary ghostname not in PATH: %v", err)
	}

	// R3.1: compare stdout and exit codes via RunDiffTests.
	// R3.2: cover normal invocation, extra operand error, unknown flag error.
	// R3.3: all tests set LC_ALL=C.
	tests := []testutils.DiffTest{
		{
			Name: "no arguments",
			Args: []string{},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name:     "extra operand",
			Args:     []string{"extraarg"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		{
			Name:     "unknown short flag",
			Args:     []string{"-x"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		{
			Name:     "unknown long flag",
			Args:     []string{"--bogus"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
