// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd015-basename R4.1–R4.3 (differential tests)
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for basename.
const refBinaryName = "gbasename"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R1.1: simple path — strip directory component.
		{
			Name: "simple_path",
			Args: []string{"/usr/bin/sort"},
		},
		// R1.1: relative path with directory.
		{
			Name: "relative_path",
			Args: []string{"include/stdio.h"},
		},
		// R1.1: no directory component — return as-is.
		{
			Name: "no_directory",
			Args: []string{"hello"},
		},
		// R1.2: suffix removal.
		{
			Name: "suffix_removal",
			Args: []string{"include/stdio.h", ".h"},
		},
		// R1.2: suffix that does not match — no change.
		{
			Name: "suffix_no_match",
			Args: []string{"include/stdio.h", ".c"},
		},
		// R1.2: suffix equals the entire base — no removal.
		{
			Name: "suffix_equals_base",
			Args: []string{"dir/.h", ".h"},
		},
		// R1.3: trailing slashes stripped before processing.
		{
			Name: "trailing_slashes",
			Args: []string{"/usr/bin/sort///"},
		},
		// R1.4: all slashes input.
		{
			Name: "all_slashes",
			Args: []string{"///"},
		},
		// R1.4: single slash.
		{
			Name: "single_slash",
			Args: []string{"/"},
		},
		// R1.5: empty string.
		{
			Name: "empty_string",
			Args: []string{""},
		},
		// R1.1: deep path.
		{
			Name: "deep_path",
			Args: []string{"/a/b/c/d/e/file.txt"},
		},
		// R1.2: suffix with dot.
		{
			Name: "suffix_dot_tar_gz",
			Args: []string{"archive.tar.gz", ".tar.gz"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffMultiple(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R2.1: -a processes multiple operands.
		{
			Name: "multiple_short_flag",
			Args: []string{"-a", "/usr/bin/sort", "/usr/bin/cat"},
		},
		// R2.1: --multiple long form.
		{
			Name: "multiple_long_flag",
			Args: []string{"--multiple", "/usr/bin/sort", "/usr/bin/cat"},
		},
		// R2.1: -a with single operand.
		{
			Name: "multiple_single_operand",
			Args: []string{"-a", "/usr/bin/sort"},
		},
		// R2.2: -s SUFFIX strips suffix from each operand.
		{
			Name: "suffix_flag_short",
			Args: []string{"-s", ".h", "include/stdio.h", "include/stdlib.h"},
		},
		// R2.2: --suffix=SUFFIX long form.
		{
			Name: "suffix_flag_long",
			Args: []string{"--suffix=.h", "include/stdio.h", "include/stdlib.h"},
		},
		// R2.2: -s implies -a, second positional is a NAME not a SUFFIX.
		{
			Name: "suffix_implies_multiple",
			Args: []string{"-s", ".txt", "file1.txt", "file2.txt"},
		},
		// R2.3: -a without -s performs no suffix removal.
		{
			Name: "multiple_no_suffix",
			Args: []string{"-a", "file1.txt", "file2.txt"},
		},
		// R2.1: -a with trailing slashes.
		{
			Name: "multiple_trailing_slashes",
			Args: []string{"-a", "/usr/bin/sort///", "/tmp/"},
		},
		// R2.1: -a with empty string operand.
		{
			Name: "multiple_empty_string",
			Args: []string{"-a", "", "hello"},
		},
		// R2.2: -a -s combined flags.
		{
			Name: "multiple_and_suffix_flags",
			Args: []string{"-a", "-s", ".txt", "file1.txt", "file2.txt"},
		},
		// R2.2: suffix that does not match in multi mode.
		{
			Name: "suffix_flag_no_match",
			Args: []string{"-s", ".c", "file1.txt", "file2.txt"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffZero(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R3.1/R2.3: -z in single operand mode.
		{
			Name: "zero_single",
			Args: []string{"-z", "/usr/bin/sort"},
		},
		// R3.1/R2.3: --zero long form.
		{
			Name: "zero_long_flag",
			Args: []string{"--zero", "/usr/bin/sort"},
		},
		// R3.1/R2.3: -z with suffix removal (two-operand form).
		{
			Name: "zero_with_suffix",
			Args: []string{"-z", "include/stdio.h", ".h"},
		},
		// R3.1/R2.3: -z combined with -a.
		{
			Name: "zero_multiple",
			Args: []string{"-z", "-a", "/usr/bin/sort", "/usr/bin/cat"},
		},
		// R3.1/R2.3: -z combined with -s.
		{
			Name: "zero_with_suffix_flag",
			Args: []string{"-z", "-s", ".txt", "file1.txt", "file2.txt"},
		},
		// R3.1/R2.3: combined short flags -az.
		{
			Name: "combined_az",
			Args: []string{"-az", "/usr/bin/sort", "/usr/bin/cat"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffHelpVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// --help exits 0.
		{
			Name: "help_flag",
			Args: []string{"--help"},
		},
		// --version exits 0.
		{
			Name: "version_flag",
			Args: []string{"--version"},
		},
	}

	// --help and --version produce different output between implementations,
	// so we only compare exit codes by normalizing stdout/stderr to empty.
	clearOutput := func(b []byte) []byte { return nil }

	for i := range tests {
		tests[i].Normalize = []testutils.NormalizeFunc{clearOutput}
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffErrors(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// R3.3, R4.1, R4.3: error cases — invalid args, missing operands.
	tests := []testutils.DiffTest{
		{
			Name:     "no_args",
			Args:     []string{},
			ExitCode: 1,
		},
		{
			Name:     "too_many_args",
			Args:     []string{"a", "b", "c"},
			ExitCode: 1,
		},
		// R4.1: -a with no operands is missing operand.
		{
			Name:     "multiple_no_operands",
			Args:     []string{"-a"},
			ExitCode: 1,
		},
		// R4.1: invalid short option.
		{
			Name:     "invalid_short_option",
			Args:     []string{"-x"},
			ExitCode: 1,
		},
		// R4.1: unrecognized long option.
		{
			Name:     "invalid_long_option",
			Args:     []string{"--invalid"},
			ExitCode: 1,
		},
	}

	// Error messages differ between implementations; normalize stderr.
	clearOutput := func(b []byte) []byte { return nil }
	for i := range tests {
		tests[i].Normalize = []testutils.NormalizeFunc{clearOutput}
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
