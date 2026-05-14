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
			Name: "multiple_args",
			Args: []string{"dir1/file", "dir2/file"},
		},
		{
			Name: "nul_delimited",
			Args: []string{"-z", "/usr/bin/sort"},
		},
		{
			Name:      "no_args_error",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
