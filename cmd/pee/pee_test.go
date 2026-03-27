// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/pee.
// Implements prd113-pee R2.1, R2.2.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("pee")
	if err != nil {
		t.Skip("reference binary pee not in PATH")
	}

	tests := []testutils.DiffTest{
		{
			// R2.2: no arguments reads stdin and exits 0.
			Name:     "no_args_stdin_consumed",
			Args:     []string{},
			Stdin:    []byte("hello\n"),
			ExitCode: 0,
		},
		{
			// R2.1: all commands succeed, exit 0.
			Name:     "all_commands_succeed",
			Args:     []string{"cat", "cat"},
			Stdin:    []byte("ok\n"),
			ExitCode: 0,
		},
		{
			// R2.1: command fails, exit 1.
			Name:     "command_fails_exit_nonzero",
			Args:     []string{"exit 1"},
			Stdin:    []byte("data\n"),
			ExitCode: 1,
		},
		{
			// R2.1: one command succeeds, one fails.
			Name:     "mixed_success_and_failure",
			Args:     []string{"true", "false"},
			Stdin:    []byte("x\n"),
			ExitCode: 1,
		},
		{
			// R2.2: no args, empty stdin.
			Name:     "no_args_empty_stdin",
			Args:     []string{},
			Stdin:    []byte{},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
