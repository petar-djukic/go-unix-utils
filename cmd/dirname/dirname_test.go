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

// versionNormalizer replaces all version output with a fixed string so
// that GNU's multi-line copyright block and Go's single-line version match.
var versionNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	if len(data) > 0 {
		return []byte("VERSION OUTPUT")
	}
	return data
}

// helpNormalizer replaces all help output with a fixed string so that
// structural differences between GNU and Go help text don't cause failures.
var helpNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	if len(data) > 0 {
		return []byte("HELP OUTPUT")
	}
	return data
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
		// R4.2: simple path (dir/file).
		{
			Name: "simple_path",
			Args: []string{"/usr/bin/sort"},
		},
		// R4.2: nested path (a/b/c).
		{
			Name: "nested_path",
			Args: []string{"/a/b/c"},
		},
		// R4.2: trailing slashes.
		{
			Name: "trailing_slashes",
			Args: []string{"/usr/bin/"},
		},
		// R4.2: root path (/).
		{
			Name: "root_path",
			Args: []string{"/"},
		},
		// R4.2: all slashes.
		{
			Name: "all_slashes",
			Args: []string{"////"},
		},
		// R4.2: relative path with no directory (file.txt -> '.').
		{
			Name: "relative_no_dir",
			Args: []string{"stdio.h"},
		},
		// R4.2: dot path (.).
		{
			Name: "dot",
			Args: []string{"."},
		},
		// R4.2: double-dot path (..).
		{
			Name: "dotdot",
			Args: []string{".."},
		},
		// R4.2: multiple arguments.
		{
			Name: "multiple_args",
			Args: []string{"/usr/bin/sort", "stdio.h", "/a/b/c"},
		},
		// R4.2: empty string argument.
		{
			Name: "empty_string",
			Args: []string{""},
		},
		// R4.3: no arguments — error output and exit code 1.
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

// TestVersion verifies that --version prints version info to stdout
// and exits 0. Implements R4.1.
func TestVersion(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdirname")
	if err != nil {
		t.Skipf("reference binary gdirname not in PATH: %v", err)
	}
	verNorm := []testutils.NormalizeFunc{versionNormalizer}
	tests := []testutils.DiffTest{
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: verNorm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestHelp verifies that --help prints usage to stdout and exits 0.
// Implements R4.2.
func TestHelp(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdirname")
	if err != nil {
		t.Skipf("reference binary gdirname not in PATH: %v", err)
	}
	helpNorm := []testutils.NormalizeFunc{helpNormalizer}
	tests := []testutils.DiffTest{
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: helpNorm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
