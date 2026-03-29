// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/whoami against gwhoami (GNU coreutils).
//
// Covers prd042-whoami R1.1, R1.2, R2.1, R2.2, R3.1, R3.2, R3.3.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
// Used for --help/--version (GNU includes paths and boilerplate) and error
// messages (GNU includes full binary path in program name).
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwhoami")
	if err != nil {
		t.Skip("reference binary gwhoami not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1, R1.2: prints effective username
		{
			Name:     "R1.1_default_username",
			Args:     []string{},
			ExitCode: 0,
		},
		// R2.1: extra operand exits 1
		{
			Name:      "R2.1_extra_operand",
			Args:      []string{"extraarg"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R2.2: unknown flag exits 1
		{
			Name:      "R2.2_unknown_flag",
			Args:      []string{"--invalid"},
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
