// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/dirname against gdirname (GNU coreutils).
//
// Covers prd016-dirname R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R3.1, R3.2, R4.1, R4.2, R4.3.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
// Used for --help and error messages where GNU includes paths and boilerplate.
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdirname")
	if err != nil {
		t.Skip("reference binary gdirname not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1: simple path — strip last component
		{
			Name:     "R1.1_simple_path",
			Args:     []string{"/usr/bin/sort"},
			ExitCode: 0,
		},
		// R1.1: nested path
		{
			Name:     "R1.1_nested_path",
			Args:     []string{"a/b/c"},
			ExitCode: 0,
		},
		// R1.2: no slash — output "."
		{
			Name:     "R1.2_no_slash",
			Args:     []string{"stdio.h"},
			ExitCode: 0,
		},
		// R1.2: single filename
		{
			Name:     "R1.2_single_name",
			Args:     []string{"file.txt"},
			ExitCode: 0,
		},
		// R1.3: trailing slashes stripped
		{
			Name:     "R1.3_trailing_slash",
			Args:     []string{"/usr/bin/"},
			ExitCode: 0,
		},
		// R1.3: multiple trailing slashes
		{
			Name:     "R1.3_multiple_trailing_slashes",
			Args:     []string{"/usr/bin///"},
			ExitCode: 0,
		},
		// R1.3: root path
		{
			Name:     "R1.3_root",
			Args:     []string{"/"},
			ExitCode: 0,
		},
		// R1.3: all slashes
		{
			Name:     "R1.3_all_slashes",
			Args:     []string{"//"},
			ExitCode: 0,
		},
		// R1.3: triple slashes
		{
			Name:     "R1.3_triple_slashes",
			Args:     []string{"///"},
			ExitCode: 0,
		},
		// R1.2: dot path
		{
			Name:     "R1.2_dot",
			Args:     []string{"."},
			ExitCode: 0,
		},
		// R1.2: double-dot path
		{
			Name:     "R1.2_dotdot",
			Args:     []string{".."},
			ExitCode: 0,
		},
		// R1.5: multiple arguments
		{
			Name:     "R1.5_multiple_args",
			Args:     []string{"/usr/bin/sort", "/usr/lib/"},
			ExitCode: 0,
		},
		// R1.5: multiple args with no-dir names
		{
			Name:     "R1.5_multiple_no_dir",
			Args:     []string{"dir1/file", "dir2/file"},
			ExitCode: 0,
		},
		// R1.2: empty string
		{
			Name:     "R1.2_empty_string",
			Args:     []string{""},
			ExitCode: 0,
		},
		// R2.1: -z NUL-delimited output
		{
			Name:     "R2.1_zero_flag",
			Args:     []string{"-z", "/usr/bin"},
			ExitCode: 0,
		},
		// R2.1: --zero long form
		{
			Name:     "R2.1_zero_long",
			Args:     []string{"--zero", "/usr/bin/sort"},
			ExitCode: 0,
		},
		// R2.1: -z with multiple args
		{
			Name:     "R2.1_zero_multiple",
			Args:     []string{"-z", "/usr/bin/sort", "/usr/lib/"},
			ExitCode: 0,
		},
		// --help prints usage and exits 0
		{
			Name:      "help",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// --version prints version and exits 0
		{
			Name:      "version",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R3.2: no arguments — error exit 1
		{
			Name:      "R3.2_no_args",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
