// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/pee against the Homebrew moreutils reference binary.
// Implements srd113-pee acceptance criteria.
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
			// R1.1: single command receives stdin
			Name:  "single_cat",
			Args:  []string{"cat"},
			Stdin: []byte("hello\n"),
		},
		{
			// R1.1, R1.3: multiple commands receive same stdin
			Name:  "two_cats",
			Args:  []string{"cat", "cat"},
			Stdin: []byte("hello\n"),
		},
		{
			// R1.1: command with arguments via sh -c
			Name:  "wc_c",
			Args:  []string{"wc -c"},
			Stdin: []byte("hello\n"),
		},
		{
			// R1.1: empty stdin
			Name:  "empty_stdin",
			Args:  []string{"cat"},
			Stdin: []byte(""),
		},
		{
			// R1.1: multi-line stdin
			Name:  "multiline",
			Args:  []string{"cat"},
			Stdin: []byte("line1\nline2\nline3\n"),
		},
		{
			// R2.1: exit 1 when command fails
			Name:     "failing_command",
			Args:     []string{"false"},
			Stdin:    []byte(""),
			ExitCode: 1,
		},
		{
			// R1.2: no commands means exit 0
			Name:  "no_commands",
			Args:  []string{},
			Stdin: []byte("hello\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
