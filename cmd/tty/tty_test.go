// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tty against gtty (GNU coreutils).
//
// Covers prd052-tty R2.2, R3.1, R3.2, R3.3.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
// Used for cases where error message text differs between implementations.
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtty")
	if err != nil {
		t.Skip("reference binary gtty not in PATH")
	}

	tests := []testutils.DiffTest{
		// R3.2: stdin redirected (not a tty) — prints "not a tty", exits 1
		{
			Name:     "not_a_tty",
			Args:     []string{},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R3.2: -s flag with stdin not a tty — no output, exits 1
		{
			Name:     "silent_not_a_tty",
			Args:     []string{"-s"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R3.2: --silent long form with stdin not a tty
		{
			Name:     "long_silent_not_a_tty",
			Args:     []string{"--silent"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R3.2: --quiet long form with stdin not a tty
		{
			Name:     "long_quiet_not_a_tty",
			Args:     []string{"--quiet"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R2.1: extra operand — error, exits 2
		{
			Name:      "extra_operand",
			Args:      []string{"extraarg"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R2.2: unknown short flag — error, exits 2
		{
			Name:      "unknown_short_flag",
			Args:      []string{"-z"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R2.2: unknown long flag — error, exits 2
		{
			Name:      "unknown_long_flag",
			Args:      []string{"--bogus"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
