// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/basename against gbasename (GNU coreutils).
//
// Covers prd015-basename R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3, R3.1, R4.1, R4.2, R4.3.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
// Used for --help (GNU includes paths and boilerplate) and error messages
// (GNU includes full binary path in program name).
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbasename")
	if err != nil {
		t.Skip("reference binary gbasename not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1: simple path — strip directory prefix
		{
			Name:     "R1.1_simple_path",
			Args:     []string{"/usr/bin/sort"},
			ExitCode: 0,
		},
		// R1.1: no directory component
		{
			Name:     "R1.1_no_dir",
			Args:     []string{"filename.txt"},
			ExitCode: 0,
		},
		// R1.2: suffix removal
		{
			Name:     "R1.2_suffix_removal",
			Args:     []string{"include/stdio.h", ".h"},
			ExitCode: 0,
		},
		// R1.2: suffix does not match — no removal
		{
			Name:     "R1.2_suffix_no_match",
			Args:     []string{"include/stdio.h", ".c"},
			ExitCode: 0,
		},
		// R1.2: suffix equals basename — no removal (would produce empty)
		{
			Name:     "R1.2_suffix_equals_name",
			Args:     []string{"stdio.h", "stdio.h"},
			ExitCode: 0,
		},
		// R1.5: suffix stripping with non-matching suffix
		{
			Name:     "R1.5_suffix_no_match_go",
			Args:     []string{"/usr/bin/sort", ".go"},
			ExitCode: 0,
		},
		// R1.5: suffix stripping with matching suffix
		{
			Name:     "R1.5_suffix_match_txt",
			Args:     []string{"file.txt", ".txt"},
			ExitCode: 0,
		},
		// R1.3: trailing slashes stripped
		{
			Name:     "R1.3_trailing_slashes",
			Args:     []string{"/usr/bin/"},
			ExitCode: 0,
		},
		// R1.3: multiple trailing slashes
		{
			Name:     "R1.3_multiple_trailing_slashes",
			Args:     []string{"/usr/bin///"},
			ExitCode: 0,
		},
		// R1.4: root path (single slash)
		{
			Name:     "R1.4_root_path",
			Args:     []string{"/"},
			ExitCode: 0,
		},
		// R1.4: multiple slashes
		{
			Name:     "R1.4_all_slashes",
			Args:     []string{"///"},
			ExitCode: 0,
		},
		// R1.5: empty string
		{
			Name:     "R1.5_empty_string",
			Args:     []string{""},
			ExitCode: 0,
		},
		// R2.1: -a multiple mode
		{
			Name:     "R2.1_multiple_a",
			Args:     []string{"-a", "/usr/bin/sort", "/usr/bin/cat"},
			ExitCode: 0,
		},
		// R2.1: --multiple long form
		{
			Name:     "R2.1_multiple_long",
			Args:     []string{"--multiple", "/usr/bin/sort", "/usr/bin/cat"},
			ExitCode: 0,
		},
		// R2.2: -s suffix with multiple names
		{
			Name:     "R2.2_suffix_s_flag",
			Args:     []string{"-s", ".go", "main.go", "util.go"},
			ExitCode: 0,
		},
		// R2.2: --suffix= long form
		{
			Name:     "R2.2_suffix_long_eq",
			Args:     []string{"--suffix=.h", "include/stdio.h", "include/stdlib.h"},
			ExitCode: 0,
		},
		// R2.2: --suffix separate arg
		{
			Name:     "R2.2_suffix_long_sep",
			Args:     []string{"--suffix", ".h", "include/stdio.h"},
			ExitCode: 0,
		},
		// R2.3: -a without -s — no suffix removal
		{
			Name:     "R2.3_multiple_no_suffix",
			Args:     []string{"-a", "file.txt", "data.csv"},
			ExitCode: 0,
		},
		// R3.1: -z NUL-delimited output
		{
			Name:     "R3.1_zero_single",
			Args:     []string{"-z", "foo"},
			ExitCode: 0,
		},
		// R3.1: -z with -a
		{
			Name:     "R3.1_zero_multiple",
			Args:     []string{"-za", "/usr/bin/sort", "/usr/bin/cat"},
			ExitCode: 0,
		},
		// R3.1: -z with -s
		{
			Name:     "R3.1_zero_suffix",
			Args:     []string{"-z", "-s", ".go", "main.go"},
			ExitCode: 0,
		},
		// --help prints usage and exits 0
		{
			Name:      "help",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// missing operand — error exit 1
		{
			Name:      "no_args_error",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.3: extra operand in single mode
		{
			Name:      "extra_operand_error",
			Args:      []string{"a", "b", "c"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// D3: suffix equals entire basename — no removal
		{
			Name:     "D3_suffix_is_entire_name",
			Args:     []string{".txt", ".txt"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
