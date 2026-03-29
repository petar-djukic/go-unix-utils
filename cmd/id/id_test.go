// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/id against gid (GNU coreutils).
//
// Covers prd041-id R1.1, R1.2, R1.3, R2.1.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
// Used for --help/--version where GNU includes paths and boilerplate.
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gid")
	if err != nil {
		t.Skip("reference binary gid not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1, R1.2, R1.3: default output uid=N(name) gid=N(name) groups=...
		{
			Name:     "R1.1_R1.2_R1.3_default_output",
			Args:     []string{},
			ExitCode: 0,
		},
		// R2.1: -u prints effective UID
		{
			Name:     "R2.1_u_flag",
			Args:     []string{"-u"},
			ExitCode: 0,
		},
		// R2.1: --user prints effective UID
		{
			Name:     "R2.1_user_long_flag",
			Args:     []string{"--user"},
			ExitCode: 0,
		},
		// --help exits 0
		{
			Name:      "help",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// --version exits 0
		{
			Name:      "version",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
