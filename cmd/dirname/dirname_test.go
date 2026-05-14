// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdirname")
	if err != nil {
		t.Skip("reference binary gdirname not found")
	}

	discardStdout := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?dirname`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("dirname"))
	})

	tests := []testutils.DiffTest{
		{
			Name: "simple_path",
			Args: []string{"/usr/bin/sort"},
		},
		{
			Name: "no_directory",
			Args: []string{"stdio.h"},
		},
		{
			Name: "root_path",
			Args: []string{"/"},
		},
		{
			Name: "trailing_slashes",
			Args: []string{"/usr/bin/"},
		},
		{
			Name: "dot_path",
			Args: []string{"."},
		},
		{
			Name: "dotdot_path",
			Args: []string{".."},
		},
		{
			Name: "nested_path",
			Args: []string{"a/b/c"},
		},
		{
			Name: "all_slashes",
			Args: []string{"///"},
		},
		{
			Name: "multiple_args",
			Args: []string{"dir1/file", "dir2/file"},
		},
		{
			Name: "multiple_args_mixed",
			Args: []string{"/usr/bin/sort", "stdio.h", "/", "a/b/c"},
		},
		{
			Name: "nul_delimited",
			Args: []string{"-z", "/usr/bin/sort"},
		},
		{
			Name: "nul_delimited_multiple",
			Args: []string{"-z", "/usr/bin/sort", "stdio.h"},
		},
		{
			Name: "nul_delimited_long_flag",
			Args: []string{"--zero", "/usr/bin/sort"},
		},
		{
			Name: "end_of_options_with_help",
			Args: []string{"--", "--help"},
		},
		{
			Name: "end_of_options_with_dash_z",
			Args: []string{"--", "-z"},
		},
		{
			Name:      "no_args_error",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name:      "help",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
		{
			Name:      "version",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
		{
			Name:      "unknown_option",
			Args:      []string{"--invalid-option"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
