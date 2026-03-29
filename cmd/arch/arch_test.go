// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/arch against garch (GNU coreutils).
//
// Covers prd045-arch R3.1, R3.2, R3.3.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
// Used for error messages where GNU includes the full binary path
// in the program name prefix, causing unavoidable divergence.
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("garch")
	if err != nil {
		t.Skip("reference binary garch not in PATH")
	}

	// R3.3: all tests set LC_ALL=C.
	env := []string{"LC_ALL=C"}

	tests := []testutils.DiffTest{
		// R3.2: normal invocation — no arguments, prints machine hardware name.
		{
			Name:     "default_no_args",
			Args:     []string{},
			Env:      env,
			ExitCode: 0,
		},
		// R3.2: extra operand error — exit 1.
		{
			Name:      "extra_operand_error",
			Args:      []string{"extraarg"},
			Env:       env,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R3.2: unknown flag error — exit 1.
		{
			Name:      "unknown_flag_error",
			Args:      []string{"--unknown"},
			Env:       env,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
