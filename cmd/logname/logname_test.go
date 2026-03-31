// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/logname implementing prd053-logname R3.1, R3.2, R3.3.
package main_test

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer replaces the reference binary path with the program name
// so stderr messages can be compared across binaries.
func stderrNormalizer() testutils.NormalizeFunc {
	re := regexp.MustCompile(`(?:glogname|/[^\s:]+/glogname)`)
	return func(b []byte) []byte {
		return re.ReplaceAll(b, []byte("logname"))
	}
}

// TestDiff runs differential tests comparing our logname against glogname.
// R3.1: compares stdout and exit codes via pkg/testutils.RunDiffTests.
// R3.2: covers normal invocation, extra operand error, and unknown flag error.
// R3.3: all tests set LC_ALL=C.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("glogname")
	if err != nil {
		t.Skipf("reference binary glogname not in PATH: %v", err)
	}

	normalize := []testutils.NormalizeFunc{stderrNormalizer()}

	tests := []testutils.DiffTest{
		{
			// R1.1, R3.2: normal invocation with no arguments
			Name:      "no_args",
			Args:      []string{},
			Env:       []string{"LC_ALL=C"},
			Normalize: normalize,
		},
		{
			// R2.1, R3.2: extra operand error
			Name:      "extra_operand",
			Args:      []string{"foo"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalize,
		},
		{
			// R2.2, R3.2: unknown flag error
			Name:      "unknown_flag",
			Args:      []string{"--bogus"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalize,
		},
		{
			// R2.1: multiple extra operands
			Name:      "two_extra_operands",
			Args:      []string{"foo", "bar"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalize,
		},
		{
			// R2.2: unknown short flag
			Name:      "unknown_short_flag",
			Args:      []string{"-x"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalize,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
