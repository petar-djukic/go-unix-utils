// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/true against gtrue (GNU coreutils).
//
// Covers prd013-true R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R3.1, R3.2, R4.1, R4.2, R4.3.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardStdout blanks all stdout so that tests comparing --help or --version
// check only exit code and stderr. GNU true's output includes full binary paths,
// OSC hyperlinks, and boilerplate that cannot be reproduced exactly.
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
		// R1.2, R2.3: single unrecognized flag ignored — exit 0, no output
		{
			Name:     "R2.3_unrecognized_flag",
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
		// R2.2, R4.2: --version prints version info and exits 0
		{
			Name:      "R2.2_version",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
		// R1.2: --version followed by other args — still exits 0
		{
			Name:      "R2.2_version_with_extra_args",
			Args:      []string{"--version", "--extra"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
		// R3.1: multiple flags ignored — exit 0
		{
			Name:     "R3.1_multiple_flags",
			Args:     []string{"-x", "-y", "-z"},
			ExitCode: 0,
		},
		// R4.3: --help as non-first arg is ignored — no output, exit 0
		{
			Name:     "R4.3_help_not_first",
			Args:     []string{"foo", "--help"},
			ExitCode: 0,
		},
		// R4.3: stdin provided but not read — exit 0, no output
		{
			Name:     "R4.3_stdin_ignored",
			Args:     []string{},
			Stdin:    []byte("input that should be ignored\n"),
			ExitCode: 0,
		},
		// R4.1: single dash argument ignored — exit 0
		{
			Name:     "R4.1_single_dash",
			Args:     []string{"-"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
