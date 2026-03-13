// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd015-basename R4.1–R4.3 (differential tests)
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for basename.
const refBinaryName = "gbasename"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R1.1: simple path — strip directory component.
		{
			Name: "simple_path",
			Args: []string{"/usr/bin/sort"},
		},
		// R1.1: relative path with directory.
		{
			Name: "relative_path",
			Args: []string{"include/stdio.h"},
		},
		// R1.1: no directory component — return as-is.
		{
			Name: "no_directory",
			Args: []string{"hello"},
		},
		// R1.2: suffix removal.
		{
			Name: "suffix_removal",
			Args: []string{"include/stdio.h", ".h"},
		},
		// R1.2: suffix that does not match — no change.
		{
			Name: "suffix_no_match",
			Args: []string{"include/stdio.h", ".c"},
		},
		// R1.2: suffix equals the entire base — no removal.
		{
			Name: "suffix_equals_base",
			Args: []string{"dir/.h", ".h"},
		},
		// R1.3: trailing slashes stripped before processing.
		{
			Name: "trailing_slashes",
			Args: []string{"/usr/bin/sort///"},
		},
		// R1.4: all slashes input.
		{
			Name: "all_slashes",
			Args: []string{"///"},
		},
		// R1.4: single slash.
		{
			Name: "single_slash",
			Args: []string{"/"},
		},
		// R1.5: empty string.
		{
			Name: "empty_string",
			Args: []string{""},
		},
		// R1.1: deep path.
		{
			Name: "deep_path",
			Args: []string{"/a/b/c/d/e/file.txt"},
		},
		// R1.2: suffix with dot.
		{
			Name: "suffix_dot_tar_gz",
			Args: []string{"archive.tar.gz", ".tar.gz"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffHelpVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R1.4: --help exits 0.
		{
			Name: "help_flag",
			Args: []string{"--help"},
		},
		// R1.4: --version exits 0.
		{
			Name: "version_flag",
			Args: []string{"--version"},
		},
	}

	// --help and --version produce different output between implementations,
	// so we only compare exit codes by normalizing stdout/stderr to empty.
	clearOutput := func(b []byte) []byte { return nil }

	for i := range tests {
		tests[i].Normalize = []testutils.NormalizeFunc{clearOutput}
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

	// R3.3, R4.3: error on no arguments — exits 1.
	tests := []testutils.DiffTest{
		{
			Name:     "no_args",
			Args:     []string{},
			ExitCode: 1,
		},
		{
			Name:     "too_many_args",
			Args:     []string{"a", "b", "c"},
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
