// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd013-true R4.1–R4.3 (differential tests)
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for true.
const refBinaryName = "gtrue"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R1.1: no arguments — exit 0, no output.
		{
			Name: "no_args",
			Args: []string{},
		},
		// R1.2: arbitrary positional arguments — still exit 0, no output.
		{
			Name: "single_arg",
			Args: []string{"foo"},
		},
		{
			Name: "multiple_args",
			Args: []string{"foo", "bar", "baz"},
		},
		// R1.2: flag-like arguments are ignored, not parsed.
		{
			Name: "flag_like_args",
			Args: []string{"--bar", "-x", "--unknown=value"},
		},
		// R1.3: verify no output on stdout or stderr (implicit via diff harness).
		{
			Name: "empty_string_arg",
			Args: []string{""},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
