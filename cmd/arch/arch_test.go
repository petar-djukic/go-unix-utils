// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd045-arch R3.1-R3.3.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath("garch")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		{Name: "no_args", ExitCode: 0},
		{Name: "extra_operand", Args: []string{"extraarg"}, ExitCode: 1},
		{Name: "unknown_flag", Args: []string{"--unknown"}, ExitCode: 1},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
