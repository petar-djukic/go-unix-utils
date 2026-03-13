// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd016-dirname R4.1–R4.3 (differential tests for R1.1–R1.4)
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for dirname.
const refBinaryName = "gdirname"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R1.1: simple path — strip last component.
		{
			Name: "simple_path",
			Args: []string{"/usr/bin/sort"},
		},
		// R1.1: nested path.
		{
			Name: "nested_path",
			Args: []string{"/a/b/c"},
		},
		// R1.1: relative path with directory.
		{
			Name: "relative_path",
			Args: []string{"dir/file"},
		},
		// R1.2: no slash — output dot.
		{
			Name: "no_slash",
			Args: []string{"file.txt"},
		},
		// R1.2: dot path.
		{
			Name: "dot_path",
			Args: []string{"."},
		},
		// R1.2: double-dot path.
		{
			Name: "double_dot_path",
			Args: []string{".."},
		},
		// R1.3: trailing slashes stripped before extraction.
		{
			Name: "trailing_slashes",
			Args: []string{"/usr/bin/"},
		},
		// R1.3: multiple trailing slashes.
		{
			Name: "multiple_trailing_slashes",
			Args: []string{"/usr/bin///"},
		},
		// R1.4: root path.
		{
			Name: "root_path",
			Args: []string{"/"},
		},
		// R1.4: multiple slashes.
		{
			Name: "all_slashes",
			Args: []string{"///"},
		},
		// R1.1: deep path.
		{
			Name: "deep_path",
			Args: []string{"/a/b/c/d/e/file.txt"},
		},
		// R1.1: path with single leading component.
		{
			Name: "single_leading_slash_component",
			Args: []string{"/usr"},
		},
		// R1.5: multiple arguments.
		{
			Name: "multiple_args",
			Args: []string{"/usr/bin/sort", "/usr/bin/cat"},
		},
		// R1.5: multiple args with mixed cases.
		{
			Name: "multiple_mixed",
			Args: []string{"file.txt", "/usr/bin/", "/", "a/b/c"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffErrors(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// R4.3: no arguments → exit 1.
	tests := []testutils.DiffTest{
		{
			Name:     "no_args",
			Args:     []string{},
			ExitCode: 1,
		},
	}

	// Error messages differ between implementations; normalize stderr.
	clearOutput := func(b []byte) []byte { return nil }
	for i := range tests {
		tests[i].Normalize = []testutils.NormalizeFunc{clearOutput}
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
