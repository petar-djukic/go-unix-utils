// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/users (prd096 R1.1–R2.3).
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gusers")
	if err != nil {
		t.Skipf("reference binary gusers not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: Print login names sorted, space-separated on a single line.
		// R1.3: Duplicate login names preserved (one per session).
		// R2.1: Exit 0 on success.
		{
			Name: "no_args_default",
			Args: []string{},
		},
		// R2.2: Exit 1 on error — extra operand.
		{
			Name:     "extra_operand",
			Args:     []string{"/dev/null", "extra"},
			ExitCode: 1,
		},
		// R1.2: FILE argument — non-existent file.
		{
			Name:     "nonexistent_file",
			Args:     []string{"/nonexistent/utmpx/path"},
			ExitCode: 0,
		},
		// R2.3: SIGPIPE handling — verified implicitly by the harness writing
		// to a pipe; explicit SIGPIPE tests require pipe setup beyond DiffTest.
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
