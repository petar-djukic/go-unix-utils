// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnice")
	if err != nil {
		t.Skipf("reference binary gnice not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		{
			// R1.1, R1.4: default adjustment (+10), passes args to command.
			Name: "default_adjustment",
			Args: []string{"echo", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R1.2: custom adjustment via -n flag.
			Name: "custom_adjustment_n",
			Args: []string{"-n", "5", "echo", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R1.3: no command prints current nice value.
			Name: "no_command",
			Args: []string{},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R1.2: custom adjustment via --adjustment= form.
			Name: "adjustment_long_equals",
			Args: []string{"--adjustment=3", "echo", "test"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R1.4: multiple arguments passed to command.
			Name: "multiple_args",
			Args: []string{"-n", "0", "echo", "a", "b", "c"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R1.2: zero adjustment.
			Name: "zero_adjustment",
			Args: []string{"-n", "0", "echo", "zero"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R1.2: --adjustment with space separator.
			Name: "adjustment_long_space",
			Args: []string{"--adjustment", "7", "echo", "spaced"},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
