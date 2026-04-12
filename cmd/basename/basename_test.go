// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/basename.
// Tests cover srd015-basename R1.1-R1.5, R2.1-R2.3, R3.1-R3.4, R4.1-R4.3.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgRe matches the program name/path prefix before a colon at line start.
var stderrProgRe = regexp.MustCompile(`(?m)^[^\s:]+:`)

// stderrTryRe matches the quoted program reference in Try hint lines.
var stderrTryRe = regexp.MustCompile(`'[^']*--help'`)

// stderrNormalizer normalizes program name differences in error messages.
// R3.4/R4.3: replaces binary paths with "PROG" so error message structure
// can be compared between Go and GNU binaries.
func stderrNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	b = stderrProgRe.ReplaceAll(b, []byte("PROG:"))
	b = stderrTryRe.ReplaceAll(b, []byte("'PROG --help'"))
	return b
}

// discardOutput normalizes by discarding all output, used when
// output content differs by design (--version, --help) and only
// exit code comparison is meaningful.
func discardOutput(b []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbasename")
	if err != nil {
		t.Skipf("reference binary gbasename not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: strip directory prefix.
		{
			Name: "simple_path",
			Args: []string{"/usr/bin/sort"},
		},
		{
			Name: "no_directory",
			Args: []string{"filename.txt"},
		},
		{
			Name: "nested_path",
			Args: []string{"/a/b/c/d/file"},
		},

		// R1.2: suffix removal.
		{
			Name: "suffix_removal",
			Args: []string{"include/stdio.h", ".h"},
		},
		{
			Name: "suffix_no_match",
			Args: []string{"file.txt", ".h"},
		},
		{
			Name: "suffix_equals_name",
			Args: []string{".h", ".h"},
		},

		// R1.3: trailing slashes stripped.
		{
			Name: "trailing_slash",
			Args: []string{"/usr/bin/"},
		},
		{
			Name: "trailing_multiple_slashes",
			Args: []string{"/usr/bin///"},
		},

		// R1.4: name entirely slashes.
		{
			Name: "root_path",
			Args: []string{"/"},
		},
		{
			Name: "multiple_slashes",
			Args: []string{"///"},
		},

		// R1.1 + R1.3 combined: path with trailing slash and suffix.
		{
			Name: "path_with_dir_and_suffix",
			Args: []string{"/home/user/file.tar.gz", ".tar.gz"},
		},

		// R1.5: empty string produces empty line.
		{
			Name: "empty_string",
			Args: []string{""},
		},

		// R2.1: -a flag processes multiple NAME arguments.
		{
			Name: "multi_arg_a_flag",
			Args: []string{"-a", "/usr/bin/sort", "/usr/bin/cat"},
		},
		{
			Name: "multi_arg_long_flag",
			Args: []string{"--multiple", "/usr/bin/sort", "/usr/bin/cat"},
		},
		{
			Name: "multi_arg_single",
			Args: []string{"-a", "/usr/bin/sort"},
		},

		// R2.2: -s SUFFIX removes suffix and implies -a.
		{
			Name: "suffix_s_flag",
			Args: []string{"-s", ".h", "include/stdio.h", "include/stdlib.h"},
		},
		{
			Name: "suffix_long_flag",
			Args: []string{"--suffix=.h", "include/stdio.h", "include/stdlib.h"},
		},
		{
			Name: "suffix_s_single_file",
			Args: []string{"-s", ".txt", "readme.txt"},
		},
		// R2.2: with -s, second positional is a NAME not a SUFFIX.
		{
			Name: "suffix_s_treats_second_as_name",
			Args: []string{"-s", ".h", "stdio.h", ".h"},
		},

		// R2.3: multi-argument mode without -s does no suffix removal.
		{
			Name: "multi_arg_no_suffix",
			Args: []string{"-a", "file.txt", "data.csv"},
		},

		// R2.1 + R1.5: multi-arg with empty string.
		{
			Name: "multi_arg_empty_string",
			Args: []string{"-a", "", "file.txt"},
		},

		// R2.1 + R1.4: multi-arg with root path.
		{
			Name: "multi_arg_root",
			Args: []string{"-a", "/", "/usr/bin/sort"},
		},

		// Combined: -az flags.
		{
			Name: "combined_az_flags",
			Args: []string{"-az", "/usr/bin/sort", "/usr/bin/cat"},
		},

		// R3.1: -z flag produces NUL-delimited output.
		{
			Name: "zero_flag_single",
			Args: []string{"-z", "/usr/bin/sort"},
		},
		{
			Name: "zero_long_flag",
			Args: []string{"--zero", "/usr/bin/sort"},
		},
		{
			Name: "zero_flag_with_suffix",
			Args: []string{"-z", "file.txt", ".txt"},
		},
		{
			Name: "zero_flag_multi_arg",
			Args: []string{"-az", "/usr/bin/sort", "/usr/bin/cat", "/usr/bin/ls"},
		},
		{
			Name: "zero_flag_with_s_suffix",
			Args: []string{"-zs", ".h", "stdio.h", "stdlib.h"},
		},

		// R3.3/R3.4: error for missing operand.
		{
			Name:      "no_args",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R3.3/R3.4: error for extra operand without -a.
		{
			Name:      "extra_operand",
			Args:      []string{"a", "b", "c"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R4.1: --version prints version info and exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R4.2: --help prints usage info and exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
