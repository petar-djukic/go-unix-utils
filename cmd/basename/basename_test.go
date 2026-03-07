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
	refBin, err := exec.LookPath("gbasename")
	if err != nil {
		t.Skipf("reference binary gbasename not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:     "simple_path",
			Args:     []string{"/usr/bin/sort"},
			ExitCode: 0,
		},
		{
			Name:     "suffix_removal",
			Args:     []string{"include/stdio.h", ".h"},
			ExitCode: 0,
		},
		{
			Name:     "trailing_slashes",
			Args:     []string{"/usr/bin/sort///"},
			ExitCode: 0,
		},
		{
			Name:     "root_path",
			Args:     []string{"/"},
			ExitCode: 0,
		},
		{
			Name:     "empty_string",
			Args:     []string{""},
			ExitCode: 0,
		},
		{
			Name:     "multi_argument_mode",
			Args:     []string{"-a", "/usr/bin/sort", "/usr/bin/cat"},
			ExitCode: 0,
		},
		{
			Name:     "suffix_mode",
			Args:     []string{"-s", ".h", "include/stdio.h", "include/stdlib.h"},
			ExitCode: 0,
		},
		{
			Name:     "nul_delimited",
			Args:     []string{"-z", "/usr/bin/sort"},
			ExitCode: 0,
		},
		{
			Name:     "no_directory",
			Args:     []string{"file.txt"},
			ExitCode: 0,
		},
		{
			Name:     "suffix_equals_name",
			Args:     []string{".h", ".h"},
			ExitCode: 0,
		},
		{
			Name:     "multiple_slashes_only",
			Args:     []string{"///"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
