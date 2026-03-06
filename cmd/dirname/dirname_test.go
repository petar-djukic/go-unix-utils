// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/dirname against gdirname (Homebrew GNU coreutils).
// Implements prd016-dirname R4.
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
		// R1.1: simple path — strip last component.
		{
			Name: "simple_path",
			Args: []string{"/usr/bin/sort"},
		},
		// R1.2: no directory component.
		{
			Name: "no_directory",
			Args: []string{"stdio.h"},
		},
		// R1.3: root path.
		{
			Name: "root_path",
			Args: []string{"/"},
		},
		// R1.1, R1.4: trailing slashes stripped before processing.
		{
			Name: "trailing_slashes",
			Args: []string{"/usr/bin/"},
		},
		// R1.2: dot path.
		{
			Name: "dot_path",
			Args: []string{"."},
		},
		// R1.2: double-dot path.
		{
			Name: "dotdot_path",
			Args: []string{".."},
		},
		// R1.5: multiple arguments.
		{
			Name: "multiple_args",
			Args: []string{"dir1/file", "dir2/file"},
		},
		// R2.1: NUL-delimited output with -z.
		{
			Name: "nul_delimited",
			Args: []string{"-z", "/usr/bin/sort"},
		},
		// Nested path.
		{
			Name: "nested_path",
			Args: []string{"a/b/c"},
		},
		// Multiple slashes only.
		{
			Name: "multiple_slashes",
			Args: []string{"///"},
		},
		// Long option --zero.
		{
			Name: "long_zero",
			Args: []string{"--zero", "/usr/bin/sort"},
		},
		// File at root.
		{
			Name: "file_at_root",
			Args: []string{"/file"},
		},
		// Multiple trailing slashes on nested path.
		{
			Name: "nested_trailing_slashes",
			Args: []string{"/usr/bin///"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
