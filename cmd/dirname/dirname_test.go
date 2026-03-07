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
	refBin, err := exec.LookPath("gdirname")
	if err != nil {
		t.Skipf("reference binary gdirname not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:     "simple_path",
			Args:     []string{"/usr/bin/sort"},
			ExitCode: 0,
		},
		{
			Name:     "no_directory",
			Args:     []string{"stdio.h"},
			ExitCode: 0,
		},
		{
			Name:     "root_path",
			Args:     []string{"/"},
			ExitCode: 0,
		},
		{
			Name:     "trailing_slashes",
			Args:     []string{"/usr/bin/"},
			ExitCode: 0,
		},
		{
			Name:     "dot_path",
			Args:     []string{"."},
			ExitCode: 0,
		},
		{
			Name:     "dotdot_path",
			Args:     []string{".."},
			ExitCode: 0,
		},
		{
			Name:     "multiple_args",
			Args:     []string{"dir1/file", "dir2/file"},
			ExitCode: 0,
		},
		{
			Name:     "nul_delimited",
			Args:     []string{"-z", "/usr/bin/sort"},
			ExitCode: 0,
		},
		{
			Name:     "nested_path",
			Args:     []string{"a/b/c"},
			ExitCode: 0,
		},
		{
			Name:     "multiple_trailing_slashes",
			Args:     []string{"/usr///"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
