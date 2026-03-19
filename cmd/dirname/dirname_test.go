// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd016-dirname R1.5, R2.1, R2.2, R3.1–R3.3,
// R4.1–R4.3: compare Go dirname against gdirname reference binary.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// binaryNameNormalizer replaces the reference binary name and path in
// stderr so that "gdirname" and "/opt/.../gdirname" both become "dirname".
var binaryNameNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	re := regexp.MustCompile(`[^\s']*g?dirname`)
	return re.ReplaceAll(data, []byte("dirname"))
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdirname")
	if err != nil {
		t.Skipf("reference binary gdirname not in PATH: %v", err)
	}
	errNorm := []testutils.NormalizeFunc{binaryNameNormalizer}
	tests := []testutils.DiffTest{
		{
			Name: "simple_path",
			Args: []string{"/usr/bin/sort"},
		},
		{
			Name: "nested_path",
			Args: []string{"/a/b/c"},
		},
		{
			Name: "trailing_slashes",
			Args: []string{"/usr/bin/"},
		},
		{
			Name: "root_path",
			Args: []string{"/"},
		},
		{
			Name: "all_slashes",
			Args: []string{"////"},
		},
		{
			Name: "relative_no_dir",
			Args: []string{"stdio.h"},
		},
		{
			Name: "dot",
			Args: []string{"."},
		},
		{
			Name: "dotdot",
			Args: []string{".."},
		},
		{
			Name: "multiple_args",
			Args: []string{"/usr/bin/sort", "stdio.h", "/a/b/c"},
		},
		{
			Name: "empty_string",
			Args: []string{""},
		},
		{
			Name:      "no_args",
			Args:      []string{},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name: "dir_file",
			Args: []string{"dir/file"},
		},
		{
			Name: "deeply_nested_trailing",
			Args: []string{"/a/b/c///"},
		},
		// R1.5: multiple NAME arguments produce one result per argument.
		{
			Name: "multiple_two_args",
			Args: []string{"/usr/bin", "/etc/hosts"},
		},
		{
			Name: "multiple_mixed_types",
			Args: []string{"/", ".", "..", "foo", "/a/b/c/"},
		},
		// R2.1: NUL-delimited output with -z flag.
		{
			Name: "zero_flag_short",
			Args: []string{"-z", "/usr/bin/sort"},
		},
		{
			Name: "zero_flag_long",
			Args: []string{"--zero", "/usr/bin/sort", "stdio.h"},
		},
		{
			Name: "zero_flag_multiple",
			Args: []string{"-z", "/a/b/c", ".", "/usr/bin/"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
