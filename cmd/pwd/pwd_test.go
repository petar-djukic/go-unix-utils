// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/pwd against gpwd (GNU coreutils).
//
// Covers prd051-pwd R3.1, R3.2, R3.3.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
// Used for error messages where GNU includes the full binary path.
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpwd")
	if err != nil {
		t.Skip("reference binary gpwd not in PATH")
	}

	tests := []testutils.DiffTest{
		// R3.2: default invocation — no flags, physical mode
		{
			Name:     "default_no_flags",
			Args:     []string{},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.2: -P explicit physical mode
		{
			Name:     "physical_P",
			Args:     []string{"-P"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.2: -L logical mode
		{
			Name:     "logical_L",
			Args:     []string{"-L"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.2: -L -P combined — last wins, physical
		{
			Name:     "L_then_P_last_wins",
			Args:     []string{"-L", "-P"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.2: -P -L combined — last wins, logical
		{
			Name:     "P_then_L_last_wins",
			Args:     []string{"-P", "-L"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.2: bundled -LP — last wins, physical
		{
			Name:     "bundled_LP",
			Args:     []string{"-LP"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.2: bundled -PL — last wins, logical
		{
			Name:     "bundled_PL",
			Args:     []string{"-PL"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.2: --physical long form
		{
			Name:     "long_physical",
			Args:     []string{"--physical"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.2: --logical long form
		{
			Name:     "long_logical",
			Args:     []string{"--logical"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1: extra operand — gpwd warns but exits 0
		{
			Name:      "extra_operand",
			Args:      []string{"foo"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R2.2: unknown short flag — error
		{
			Name:      "unknown_short_flag",
			Args:      []string{"-z"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R2.2: unknown long flag — error
		{
			Name:      "unknown_long_flag",
			Args:      []string{"--bogus"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
