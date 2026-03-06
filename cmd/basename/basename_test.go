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

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbasename")
	if err != nil {
		t.Skipf("reference binary gbasename not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: simple path stripping.
		{
			Name: "simple_path",
			Args: []string{"/usr/bin/sort"},
		},
		// R1.2: suffix removal.
		{
			Name: "suffix_removal",
			Args: []string{"include/stdio.h", ".h"},
		},
		// R1.3: trailing slashes stripped.
		{
			Name: "trailing_slashes",
			Args: []string{"/usr/bin/sort///"},
		},
		// R1.4: root path.
		{
			Name: "root_path",
			Args: []string{"/"},
		},
		// R1.5: empty string.
		{
			Name: "empty_string",
			Args: []string{""},
		},
		// R2.1: multi-argument mode with -a.
		{
			Name: "multi_argument_a",
			Args: []string{"-a", "/usr/bin/sort", "/usr/bin/cat"},
		},
		// R2.2: suffix mode with -s (implies -a).
		{
			Name: "suffix_mode_s",
			Args: []string{"-s", ".h", "include/stdio.h", "include/stdlib.h"},
		},
		// R3.1: NUL-delimited output with -z.
		{
			Name: "nul_delimited",
			Args: []string{"-z", "/usr/bin/sort"},
		},
		// Combined flags: -az with multiple args.
		{
			Name: "combined_az",
			Args: []string{"-az", "/usr/bin/sort", "/usr/bin/cat"},
		},
		// Suffix that equals the entire basename should not be removed.
		{
			Name: "suffix_equals_name",
			Args: []string{".h", ".h"},
		},
		// Relative path with no directory.
		{
			Name: "no_directory",
			Args: []string{"file.txt"},
		},
		// Multiple slashes only.
		{
			Name: "multiple_slashes",
			Args: []string{"///"},
		},
		// Long option --multiple.
		{
			Name: "long_multiple",
			Args: []string{"--multiple", "a/b", "c/d"},
		},
		// Long option --suffix=.
		{
			Name: "long_suffix",
			Args: []string{"--suffix=.h", "include/stdio.h"},
		},
		// Long option --zero.
		{
			Name: "long_zero",
			Args: []string{"--zero", "/usr/bin/sort"},
		},
		// Double-dash separator.
		{
			Name: "double_dash",
			Args: []string{"-a", "--", "-z", "/usr/bin/sort"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
