// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/dircolors against GNU gdircolors.
// Covers prd109-dircolors R1.1 (Bourne shell output), R1.2 (C shell output),
// R1.3 (shell auto-detection), R1.4 (mutually exclusive -b/-c flags).
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdircolors")
	if err != nil {
		t.Skip("reference binary gdircolors not in PATH")
	}
	tests := []testutils.DiffTest{
		// R1.1: Bourne shell output format
		{
			Name: "sh-flag",
			Args: []string{"--sh"},
			Env:  []string{"TERM=xterm-256color"},
		},
		{
			Name: "b-flag",
			Args: []string{"-b"},
			Env:  []string{"TERM=xterm-256color"},
		},
		{
			Name: "bourne-shell-long-flag",
			Args: []string{"--bourne-shell"},
			Env:  []string{"TERM=xterm-256color"},
		},
		// R1.2: C shell output format
		{
			Name: "csh-flag",
			Args: []string{"--csh"},
			Env:  []string{"TERM=xterm-256color"},
		},
		{
			Name: "c-flag",
			Args: []string{"-c"},
			Env:  []string{"TERM=xterm-256color"},
		},
		{
			Name: "c-shell-long-flag",
			Args: []string{"--c-shell"},
			Env:  []string{"TERM=xterm-256color"},
		},
		// R1.3: Auto-detect shell from SHELL env
		{
			Name: "auto-detect-bash",
			Args: []string{},
			Env:  []string{"SHELL=/bin/bash", "TERM=xterm-256color"},
		},
		{
			Name: "auto-detect-csh",
			Args: []string{},
			Env:  []string{"SHELL=/bin/csh", "TERM=xterm-256color"},
		},
		{
			Name: "auto-detect-tcsh",
			Args: []string{},
			Env:  []string{"SHELL=/bin/tcsh", "TERM=xterm-256color"},
		},
		{
			Name: "auto-detect-zsh",
			Args: []string{},
			Env:  []string{"SHELL=/bin/zsh", "TERM=xterm-256color"},
		},
		// R1.4: -b and -c mutually exclusive, last one wins
		{
			Name: "last-wins-b-then-c",
			Args: []string{"-b", "-c"},
			Env:  []string{"TERM=xterm-256color"},
		},
		{
			Name: "last-wins-c-then-b",
			Args: []string{"-c", "-b"},
			Env:  []string{"TERM=xterm-256color"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
