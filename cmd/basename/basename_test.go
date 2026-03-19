// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd015-basename R3.1–R3.4, R4.1–R4.3:
// error handling, exit codes, output formatting, and differential
// test coverage against gbasename reference binary.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// binaryNameNormalizer replaces the reference binary name and path in
// stderr so that "gbasename" and "/opt/.../gbasename" both become "basename".
var binaryNameNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	re := regexp.MustCompile(`[^\s']*g?basename`)
	return re.ReplaceAll(data, []byte("basename"))
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbasename")
	if err != nil {
		t.Skipf("reference binary gbasename not in PATH: %v", err)
	}
	errNorm := []testutils.NormalizeFunc{binaryNameNormalizer}
	tests := []testutils.DiffTest{
		// R3.1: newline-terminated output
		{
			Name: "simple_path_newline",
			Args: []string{"/usr/bin/sort"},
		},
		{
			Name: "suffix_removal_newline",
			Args: []string{"include/stdio.h", ".h"},
		},
		// R3.1: NUL-terminated output with -z
		{
			Name: "zero_flag_short",
			Args: []string{"-z", "/usr/bin/sort"},
		},
		{
			Name: "zero_flag_long",
			Args: []string{"--zero", "/usr/bin/sort"},
		},
		{
			Name: "zero_with_multiple",
			Args: []string{"-az", "/usr/bin/sort", "/usr/bin/cat"},
		},
		// R3.2: exit 0 on success
		{
			Name:     "exit_zero_simple",
			Args:     []string{"file.txt"},
			ExitCode: 0,
		},
		{
			Name:     "exit_zero_multi",
			Args:     []string{"-a", "a", "b", "c"},
			ExitCode: 0,
		},
		// R3.3: exit 1 with zero arguments
		{
			Name:      "no_args_exit_one",
			Args:      []string{},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R3.3: exit 1 with >2 args without -a
		{
			Name:      "extra_operand_exit_one",
			Args:      []string{"a", "b", "c"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R3.4: stderr error message on validation failure
		{
			Name:      "error_msg_no_args",
			Args:      []string{},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "error_msg_extra_operand",
			Args:      []string{"x", "y", "z"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R4.2: simple path (dir/file)
		{
			Name: "r4_simple_dir_file",
			Args: []string{"dir/file"},
		},
		// R4.2: trailing slashes
		{
			Name: "trailing_slashes",
			Args: []string{"/usr/bin/"},
		},
		{
			Name: "trailing_multiple_slashes",
			Args: []string{"/usr/bin///"},
		},
		// R4.2: root path
		{
			Name: "root_path",
			Args: []string{"/"},
		},
		{
			Name: "all_slashes",
			Args: []string{"////"},
		},
		// R4.2: empty string
		{
			Name: "empty_string",
			Args: []string{""},
		},
		// R4.2: suffix removal
		{
			Name: "r4_suffix_removal",
			Args: []string{"archive.tar.gz", ".tar.gz"},
		},
		{
			Name: "r4_suffix_no_match",
			Args: []string{"file.txt", ".c"},
		},
		{
			Name: "r4_suffix_equals_name",
			Args: []string{".h", ".h"},
		},
		// R4.2: multi-argument mode (-a)
		{
			Name: "r4_multi_arg_a_flag",
			Args: []string{"-a", "/usr/bin/sort", "/usr/bin/cat", "/tmp/file.txt"},
		},
		{
			Name: "r4_multi_arg_long_flag",
			Args: []string{"--multiple", "/usr/bin/sort", "/usr/bin/cat"},
		},
		// R4.2: suffix mode (-s)
		{
			Name: "r4_suffix_s_flag",
			Args: []string{"-s", ".h", "include/stdio.h", "include/stdlib.h"},
		},
		{
			Name: "r4_suffix_long_flag",
			Args: []string{"--suffix=.h", "include/stdio.h", "include/stdlib.h"},
		},
		// R4.2: NUL-delimited output (-z)
		{
			Name: "r4_zero_single",
			Args: []string{"-z", "dir/file"},
		},
		{
			Name: "r4_zero_multi_suffix",
			Args: []string{"-z", "-s", ".h", "stdio.h", "stdlib.h"},
		},
		// R4.3: error output and exit code for invalid argument counts
		{
			Name:      "r4_error_no_args",
			Args:      []string{},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "r4_error_three_args_no_flag",
			Args:      []string{"a", "b", "c"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name: "suffix_with_zero",
			Args: []string{"-z", "-s", ".h", "stdio.h", "stdlib.h"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
