// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/nproc against gnproc (GNU coreutils).
//
// Covers prd046-nproc R3.2, R3.3.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
// Used for error messages and version/help output where GNU includes
// the full binary path, causing unavoidable divergence.
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	// R3.3: skip gracefully when gnproc reference binary is not found.
	refBin, err := exec.LookPath("gnproc")
	if err != nil {
		t.Skip("reference binary gnproc not in PATH")
	}

	env := []string{"LC_ALL=C"}

	tests := []testutils.DiffTest{
		// R3.2: no arguments — prints available CPU count.
		{
			Name:     "default_no_args",
			Args:     []string{},
			Env:      env,
			ExitCode: 0,
		},
		// R3.2: --all prints installed processor count.
		{
			Name:     "flag_all",
			Args:     []string{"--all"},
			Env:      env,
			ExitCode: 0,
		},
		// R3.2: --ignore=1 subtracts from available count.
		{
			Name:     "flag_ignore_1",
			Args:     []string{"--ignore=1"},
			Env:      env,
			ExitCode: 0,
		},
		// R3.2: --all --ignore=2 combined.
		{
			Name:     "flag_all_ignore_2",
			Args:     []string{"--all", "--ignore=2"},
			Env:      env,
			ExitCode: 0,
		},
		// R3.2: --ignore=0 has no effect.
		{
			Name:     "flag_ignore_0",
			Args:     []string{"--ignore=0"},
			Env:      env,
			ExitCode: 0,
		},
		// R3.2: --ignore with large value floors to 1.
		{
			Name:     "flag_ignore_large",
			Args:     []string{"--ignore=99999"},
			Env:      env,
			ExitCode: 0,
		},
		// R3.2: --version exits 0 (output differs, check exit code only).
		{
			Name:      "flag_version",
			Args:      []string{"--version"},
			Env:       env,
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R3.2: --help exits 0 (output differs, check exit code only).
		{
			Name:      "flag_help",
			Args:      []string{"--help"},
			Env:       env,
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R3.2: unrecognized option exits 1.
		{
			Name:      "invalid_option",
			Args:      []string{"--bogus"},
			Env:       env,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestHelp verifies --help prints to stdout and exits 0.
func TestHelp(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("--help produced no output")
	}
}
