// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests: prd014-false R4.1, R4.2, R4.3 via differential testing against gfalse
// (Homebrew GNU false).
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfalse")
	if err != nil {
		t.Skipf("reference binary gfalse not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.1, R4.3: no arguments -- exits 1, stdout empty, stderr empty.
		{
			Name: "false_no_args",
		},
		// R4.2, R4.3: one extra argument ignored -- exits 1, no output.
		{
			Name: "false_one_arg",
			Args: []string{"foo"},
		},
		// R4.2, R4.3: multiple extra arguments ignored -- exits 1, no output.
		{
			Name: "false_multiple_args",
			Args: []string{"foo", "bar", "baz"},
		},
		// R4.2, R4.3: unknown flags treated as ignored arguments -- exits 1, no output.
		{
			Name: "false_unknown_flags",
			Args: []string{"--unknown", "-x", "-abc"},
		},
		// R4.2: single dash treated as an argument, ignored.
		{
			Name: "false_single_dash",
			Args: []string{"-"},
		},
		// R4.2: double dash treated as an argument, ignored.
		{
			Name: "false_double_dash",
			Args: []string{"--"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
