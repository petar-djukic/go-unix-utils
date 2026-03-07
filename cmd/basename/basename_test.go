// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/basename against gbasename (Homebrew GNU coreutils).
// Implements prd015-basename R4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinaryName = "gbasename"

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
			Name: "relative path",
			Args: []string{"dir/file.txt"},
		},
		{
			Name: "no directory",
			Args: []string{"file.txt"},
		},
		// R1.2: Suffix removal.
		{
			Name: "suffix removal",
			Args: []string{"include/stdio.h", ".h"},
		},
		{
			Name: "suffix equals basename",
			Args: []string{".h", ".h"},
		},
		{
			Name: "suffix no match",
			Args: []string{"file.txt", ".h"},
		},
		// R1.3: Trailing slashes.
		{
			Name: "trailing slashes",
			Args: []string{"/usr/bin/sort///"},
		},
		{
			Name: "trailing slash dir",
			Args: []string{"/usr/bin/"},
		},
		// R1.4: Root path.
		{
			Name: "root path",
			Args: []string{"/"},
		},
		{
			Name: "multiple slashes",
			Args: []string{"///"},
		},
		// R1.5: Empty string.
		{
			Name: "empty string",
			Args: []string{""},
		},
		// R2.1: Multi-argument mode.
		{
			Name: "multi argument mode",
			Args: []string{"-a", "/usr/bin/sort", "/usr/bin/cat"},
		},
		{
			Name: "multiple long flag",
			Args: []string{"--multiple", "/usr/bin/sort", "/usr/bin/cat"},
		},
		// R2.2: Suffix mode implies -a.
		{
			Name: "suffix mode",
			Args: []string{"-s", ".h", "include/stdio.h", "include/stdlib.h"},
		},
		{
			Name: "suffix equals form",
			Args: []string{"--suffix=.h", "include/stdio.h", "include/stdlib.h"},
		},
		// R2.3: Multi-argument mode without suffix.
		{
			Name: "multi no suffix",
			Args: []string{"-a", "include/stdio.h", "include/stdlib.h"},
		},
		// R3.1: NUL-delimited output.
		{
			Name: "nul delimited single",
			Args: []string{"-z", "/usr/bin/sort"},
		},
		{
			Name: "nul delimited multi",
			Args: []string{"-za", "/usr/bin/sort", "/usr/bin/cat"},
		},
		// Combined flags.
		{
			Name: "combined az flags",
			Args: []string{"-az", "/usr/bin/sort", "/usr/bin/cat"},
		},
		// R3.3, R3.4: Error cases — blank output because error text differs.
		{
			Name:      "no arguments error",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{blankOutput},
		},
		{
			Name:      "too many args single mode",
			Args:      []string{"a", "b", "c"},
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
