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
	refBin, err := exec.LookPath("gbasename")
	if err != nil {
		t.Skip("reference binary not found")
	}

	discardStdout := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?basename`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("basename"))
	})

	tests := []testutils.DiffTest{
		// R4.2: simple path (dir/file)
		{
			Name: "simple-path",
			Args: []string{"/usr/bin/sort"},
		},
		{
			Name: "relative-path",
			Args: []string{"dir/file.txt"},
		},
		// R4.2: trailing slashes
		{
			Name: "trailing-slash",
			Args: []string{"/usr/bin/sort/"},
		},
		{
			Name: "multiple-trailing-slashes",
			Args: []string{"/usr/bin/sort///"},
		},
		// R4.2: root path (/)
		{
			Name: "root-path",
			Args: []string{"/"},
		},
		{
			Name: "multiple-slashes",
			Args: []string{"///"},
		},
		// R4.2: empty string
		{
			Name: "empty-string",
			Args: []string{""},
		},
		// R4.2: suffix removal
		{
			Name: "suffix-removal",
			Args: []string{"include/stdio.h", ".h"},
		},
		{
			Name: "suffix-no-match",
			Args: []string{"include/stdio.h", ".c"},
		},
		{
			Name: "suffix-equals-name",
			Args: []string{".h", ".h"},
		},
		// R4.2: multi-argument mode (-a)
		{
			Name: "multi-arg",
			Args: []string{"-a", "/usr/bin/sort", "/usr/bin/cat"},
		},
		{
			Name: "multi-arg-long",
			Args: []string{"--multiple", "/usr/bin/sort", "/usr/bin/cat"},
		},
		// R4.2: suffix mode (-s)
		{
			Name: "suffix-mode",
			Args: []string{"-s", ".h", "include/stdio.h", "include/stdlib.h"},
		},
		{
			Name: "suffix-mode-long",
			Args: []string{"--suffix=.h", "include/stdio.h", "include/stdlib.h"},
		},
		// R4.2: NUL-delimited output (-z)
		{
			Name: "zero-single",
			Args: []string{"-z", "/usr/bin/sort"},
		},
		{
			Name: "zero-multi",
			Args: []string{"-az", "/usr/bin/sort", "/usr/bin/cat"},
		},
		{
			Name: "zero-long",
			Args: []string{"--zero", "/usr/bin/sort"},
		},
		// R4.3: error for missing operand
		{
			Name:      "error-no-args",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// R4.3: error for extra operand without -a
		{
			Name:      "error-extra-operand",
			Args:      []string{"a", "b", "c"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// --help and --version (discard stdout since text differs)
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
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
