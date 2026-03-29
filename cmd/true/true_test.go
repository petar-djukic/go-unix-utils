// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/true against gtrue (GNU coreutils).
//
// Covers prd013-true R1.1, R1.2, R1.3, R2.1, R4.1, R4.2, R4.3.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardStdout blanks all stdout so that the --help test compares only exit
// code and stderr. GNU true's help text includes full binary paths, OSC
// hyperlinks, and boilerplate that cannot be reproduced exactly.
func discardStdout(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtrue")
	if err != nil {
		t.Skip("reference binary gtrue not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1, R4.3: no arguments — exit 0, no output
		{
			Name:     "R1.1_no_args",
			Args:     []string{},
			ExitCode: 0,
		},
		// R1.2, R4.2: arbitrary arguments ignored — exit 0, no output
		{
			Name:     "R1.2_arbitrary_args",
			Args:     []string{"foo", "bar", "--baz"},
			ExitCode: 0,
		},
		// R1.2: single argument ignored
		{
			Name:     "R1.2_single_arg",
			Args:     []string{"--unknown"},
			ExitCode: 0,
		},
		// R2.1, R4.2: --help prints usage and exits 0
		{
			Name:      "R2.1_help",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
