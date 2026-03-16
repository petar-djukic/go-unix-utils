// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/true against gtrue (GNU coreutils).
// Implements prd013-true R1.1-R1.3, R2.1-R2.2, R3.1-R3.2, R4.1-R4.3 test coverage.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtrue")
	if err != nil {
		t.Skipf("reference binary gtrue not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1, R4.3: no arguments — exit 0, no output.
		{
			Name:     "R1.1_no_args_exit_0",
			ExitCode: 0,
		},
		// R1.2, R4.2: arbitrary arguments ignored — exit 0, no output.
		{
			Name:     "R1.2_args_ignored",
			Args:     []string{"foo", "bar", "--baz"},
			ExitCode: 0,
		},
		// R1.2: single argument ignored.
		{
			Name:     "R1.2_single_arg_ignored",
			Args:     []string{"hello"},
			ExitCode: 0,
		},
		// R1.2: flag-like arguments ignored (not --help or --version).
		{
			Name:     "R1.2_flag_like_ignored",
			Args:     []string{"-x", "-v", "--unknown"},
			ExitCode: 0,
		},
		// R2.1, R4.2: --help prints usage and exits 0.
		{
			Name:      "R2.1_help_exits_0",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeAllOutput},
		},
		// R2.2, R4.2: --version prints version info and exits 0.
		{
			Name:      "R2.2_version_exits_0",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeAllOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// normalizeAllOutput replaces all output with empty bytes so that only
// exit codes are compared. Used for --version and --help where output
// content intentionally differs between implementations.
func normalizeAllOutput(b []byte) []byte {
	return nil
}
