// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/basename.
// Tests cover srd015-basename R1.1, R1.2, R1.3, R1.4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrDropNormalizer drops stderr content so only exit code is compared
// for error cases where the binary name in the message differs.
func stderrDropNormalizer(b []byte) []byte {
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

		// Empty string input (R1.5 behavior, handled naturally).
		{
			Name: "empty_string",
			Args: []string{""},
		},

		// Error: no arguments.
		{
			Name: "no_args",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrDropNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
