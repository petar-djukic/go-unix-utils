// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/vidir.
// Implements prd114-vidir R2.1, R2.2.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("vidir")
	if err != nil {
		t.Skip("reference binary vidir not in PATH")
	}

	tests := []testutils.DiffTest{
		{
			// R2.1: empty stdin, no files to process, exit 0.
			Name:     "empty_stdin_exit_zero",
			Args:     []string{},
			Stdin:    []byte{},
			Env:      []string{"EDITOR=true"},
			ExitCode: 0,
		},
		{
			// R2.1: stdin with paths, editor makes no changes, exit 0.
			Name:     "stdin_paths_no_edit",
			Args:     []string{},
			Stdin:    []byte("foo\nbar\nbaz\n"),
			Env:      []string{"EDITOR=true"},
			ExitCode: 0,
		},
		{
			// R2.2: verbose flag with no changes produces no output.
			Name:     "verbose_no_changes",
			Args:     []string{"-v"},
			Stdin:    []byte("alpha\nbeta\n"),
			Env:      []string{"EDITOR=true"},
			ExitCode: 0,
		},
		{
			// R2.1: editor exits non-zero, vidir aborts with exit 1.
			Name:  "editor_exits_nonzero",
			Args:  []string{},
			Stdin: []byte("somefile\n"),
			Env:   []string{"EDITOR=false"},
			// R2.1: stderr messages differ between implementations, normalize away.
			Normalize: []testutils.NormalizeFunc{clearBytes},
			ExitCode:  1,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// clearBytes normalizes output to empty for cases where stderr
// messages are expected to differ between implementations.
func clearBytes(b []byte) []byte {
	return nil
}
