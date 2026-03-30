// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// pee_test.go implements differential tests for prd113-pee R1.1, R1.2, R1.3.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("pee")
	if err != nil {
		t.Skipf("reference binary pee not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R1.1: pipe stdin to a single command.
			Name:  "single_command_cat",
			Args:  []string{"cat"},
			Stdin: []byte("hello\n"),
		},
		{
			// R1.1, R1.3: pipe stdin to two commands, stdout interleaved.
			Name:  "two_commands_cat_wc",
			Args:  []string{"cat", "wc -c"},
			Stdin: []byte("hello\n"),
		},
		{
			// R1.1: no commands produces no output.
			Name:  "no_commands",
			Args:  []string{},
			Stdin: []byte("hello\n"),
		},
		{
			// R1.1: empty stdin produces no output from cat.
			Name:  "empty_stdin",
			Args:  []string{"cat"},
			Stdin: []byte{},
		},
		{
			// R1.2: wait for all commands.
			Name:  "three_commands",
			Args:  []string{"cat", "cat", "cat"},
			Stdin: []byte("abc\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
