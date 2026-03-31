// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeProgramName replaces "gnice:" with "nice:" in stderr output
// so the Go binary and reference binary error messages can be compared.
func normalizeProgramName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gnice:"), []byte("nice:"))
}

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
		{
			// R2.1: exit status propagated from command.
			Name:     "exit_status_propagated",
			Args:     []string{"sh", "-c", "exit 42"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 42,
		},
		{
			// R2.2: exit 125 for invalid adjustment.
			Name:      "invalid_adjustment",
			Args:      []string{"-n", "abc", "echo", "hi"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		{
			// R2.2: exit 127 when command not found.
			Name:      "command_not_found",
			Args:      []string{"nonexistent_command_xyz_12345"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  127,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
