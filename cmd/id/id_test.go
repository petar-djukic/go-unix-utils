// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/id against gid (GNU coreutils).
//
// Covers prd041-id R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R2.4, R3.1.
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
		// R2.2: -g prints effective GID
		{
			Name:     "R2.2_g_flag",
			Args:     []string{"-g"},
			ExitCode: 0,
		},
		// R2.2: --group prints effective GID
		{
			Name:     "R2.2_group_long_flag",
			Args:     []string{"--group"},
			ExitCode: 0,
		},
		// R2.3: -G prints all group IDs space-separated
		{
			Name:     "R2.3_G_flag",
			Args:     []string{"-G"},
			ExitCode: 0,
		},
		// R2.3: --groups prints all group IDs
		{
			Name:     "R2.3_groups_long_flag",
			Args:     []string{"--groups"},
			ExitCode: 0,
		},
		// R2.4: -u -g conflicts -> error exit 1
		{
			Name:      "R2.4_u_g_conflict",
			Args:      []string{"-u", "-g"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R2.4: -u -G conflicts -> error exit 1
		{
			Name:      "R2.4_u_G_conflict",
			Args:      []string{"-u", "-G"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R2.4: -g -G conflicts -> error exit 1
		{
			Name:      "R2.4_g_G_conflict",
			Args:      []string{"-g", "-G"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R3.1: -un prints effective user name
		{
			Name:     "R3.1_un_flag",
			Args:     []string{"-u", "-n"},
			ExitCode: 0,
		},
		// R3.1: -gn prints effective group name
		{
			Name:     "R3.1_gn_flag",
			Args:     []string{"-g", "-n"},
			ExitCode: 0,
		},
		// R3.1: -Gn prints all group names
		{
			Name:     "R3.1_Gn_flag",
			Args:     []string{"-G", "-n"},
			ExitCode: 0,
		},
		// R3.1: -n alone (no selection flag) is an error
		{
			Name:      "R3.1_n_without_selection",
			Args:      []string{"-n"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
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
