// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/nice against gnice (GNU coreutils).
// Implements srd094 R2.1 (TestDiff with RunDiffTests), R2.2 (adjustment tests),
// R2.3 (error handling and exit code tests).
package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinName = "gnice"

// makeNormalizer creates a NormalizeFunc that replaces binary names and
// normalizes syscall error message capitalization between GNU and Go.
func makeNormalizer(refBin string) testutils.NormalizeFunc {
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte(progName))
		b = bytes.ReplaceAll(b, []byte(refBinName), []byte(progName))
		b = normalizeSyscallErrors(b)
		return b
	}
}

// normalizeSyscallErrors lowercases known syscall error messages that
// differ in case between C strerror() and Go syscall.Errno.Error().
func normalizeSyscallErrors(b []byte) []byte {
	replacements := []struct{ from, to string }{
		{"Permission denied", "permission denied"},
		{"Operation not permitted", "operation not permitted"},
		{"No such file or directory", "no such file or directory"},
	}
	for _, r := range replacements {
		b = bytes.ReplaceAll(b, []byte(r.from), []byte(r.to))
	}
	return b
}

// TestDiff runs differential tests comparing cmd/nice against gnice.
// R2.1: uses testutils.BuildBinary and testutils.RunDiffTests.
// R2.2: covers default, positive, negative, zero adjustments.
// R2.3: covers invalid adjustment, non-existent command errors.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}
	norm := makeNormalizer(refBin)
	norms := []testutils.NormalizeFunc{norm}

	tests := []testutils.DiffTest{
		// R2.2: default adjustment (increment of 10). R1.1, R1.4.
		{
			Name:      "default_adjustment",
			Args:      []string{"echo", "hello"},
			Normalize: norms,
		},
		// R2.2: explicit -n positive adjustment. R1.2.
		{
			Name:      "positive_n5",
			Args:      []string{"-n", "5", "echo", "hello"},
			Normalize: norms,
		},
		// R2.2: zero adjustment. R1.2.
		{
			Name:      "zero_adjustment",
			Args:      []string{"-n", "0", "echo", "hello"},
			Normalize: norms,
		},
		// R2.2: --adjustment= long form. R1.2.
		{
			Name:      "long_adjustment_flag",
			Args:      []string{"--adjustment=7", "echo", "hello"},
			Normalize: norms,
		},
		// R2.2: -nVALUE short form (no space). R1.2.
		{
			Name:      "short_n_no_space",
			Args:      []string{"-n3", "echo", "hello"},
			Normalize: norms,
		},
		// R2.2: negative adjustment (may warn without privileges). R1.2.
		{
			Name:      "negative_adjustment",
			Args:      []string{"-n", "-1", "echo", "hello"},
			Normalize: norms,
		},
		// R2.2: no command prints current nice value. R1.3.
		{
			Name:      "no_command",
			Args:      []string{},
			Normalize: norms,
		},
		// R2.2: command with multiple arguments. R1.4.
		{
			Name:      "command_with_args",
			Args:      []string{"-n", "3", "echo", "one", "two", "three"},
			Normalize: norms,
		},
		// R2.3: non-existent command exits 127. R2.2.
		{
			Name:      "nonexistent_command",
			Args:      []string{"nonexistent_cmd_xyz_42"},
			ExitCode:  127,
			Normalize: norms,
		},
		// R2.3: invalid adjustment value exits 125. R2.2.
		{
			Name:      "invalid_adjustment",
			Args:      []string{"-n", "abc", "echo", "hello"},
			ExitCode:  125,
			Normalize: norms,
		},
		// R2.3: missing adjustment argument. R2.2.
		{
			Name:      "missing_adjustment_arg",
			Args:      []string{"-n"},
			ExitCode:  125,
			Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
