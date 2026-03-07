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

const refBinaryName = "gdirname"

// blankOutput blanks output so only exit codes are compared.
// Used for --help, --version, and error messages where text differs.
var blankOutput testutils.NormalizeFunc = func(b []byte) []byte { return nil }

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R1.1: Simple path stripping.
		{
			Name: "simple path",
			Args: []string{"/usr/bin/sort"},
		},
		{
			Name: "nested path",
			Args: []string{"/a/b/c"},
		},
		{
			Name: "relative path with dir",
			Args: []string{"dir/file.txt"},
		},
		// R1.2: No directory component.
		{
			Name: "no directory component",
			Args: []string{"file.txt"},
		},
		{
			Name: "dot path",
			Args: []string{"."},
		},
		{
			Name: "double dot path",
			Args: []string{".."},
		},
		// R1.3: Root and all-slash paths.
		{
			Name: "root path",
			Args: []string{"/"},
		},
		{
			Name: "multiple slashes",
			Args: []string{"///"},
		},
		// R1.1 + R1.4: Trailing slashes.
		{
			Name: "trailing slashes",
			Args: []string{"/usr/bin/sort///"},
		},
		{
			Name: "trailing slash dir",
			Args: []string{"/usr/bin/"},
		},
		// R1.5: Multiple arguments.
		{
			Name: "multiple arguments",
			Args: []string{"/usr/bin/sort", "/etc/hosts"},
		},
		{
			Name: "multiple with no dir",
			Args: []string{"file1", "file2"},
		},
		// Empty string argument.
		{
			Name: "empty string",
			Args: []string{""},
		},
		// R2.1: NUL-delimited output.
		{
			Name: "nul delimited single",
			Args: []string{"-z", "/usr/bin/sort"},
		},
		{
			Name: "nul delimited multi",
			Args: []string{"-z", "/usr/bin/sort", "/etc/hosts"},
		},
		{
			Name: "zero long flag",
			Args: []string{"--zero", "/usr/bin/sort"},
		},
		// R3.2: Error cases — blank output because error text differs.
		{
			Name:      "no arguments error",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{blankOutput},
		},
		// --help and --version: blank output, only exit code matters.
		{
			Name:      "help flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{blankOutput},
		},
		{
			Name:      "version flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{blankOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
