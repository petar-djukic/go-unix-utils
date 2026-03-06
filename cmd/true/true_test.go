// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/true against gtrue reference binary.
// Implements prd013-true R4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinName = "gtrue"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	tests := []testutils.DiffTest{
		{
			Name:     "no_args",
			Args:     []string{},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		{
			Name:     "arbitrary_args",
			Args:     []string{"foo", "bar", "--baz"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestHelp(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	if len(out) == 0 {
		t.Error("--help produced no output")
	}
}

func TestVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if len(out) == 0 {
		t.Error("--version produced no output")
	}
}
