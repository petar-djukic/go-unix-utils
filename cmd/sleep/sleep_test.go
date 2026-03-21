// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sleep against gsleep reference binary.
// Implements prd061-sleep R4.3, R4.4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrClear is a normalizer that clears stderr for error-case tests where
// the binary name/path in error messages causes expected divergence.
var stderrClear testutils.NormalizeFunc = func(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsleep")
	if err != nil {
		t.Skip("reference binary gsleep not in PATH")
	}

	tests := []testutils.DiffTest{
		{
			Name:     "zero_duration",
			Args:     []string{"0"},
			ExitCode: 0,
		},
		{
			Name:     "fractional_seconds",
			Args:     []string{"0.001"},
			ExitCode: 0,
		},
		{
			Name:     "suffix_s",
			Args:     []string{"0s"},
			ExitCode: 0,
		},
		{
			Name:     "suffix_m",
			Args:     []string{"0m"},
			ExitCode: 0,
		},
		{
			Name:     "suffix_h",
			Args:     []string{"0h"},
			ExitCode: 0,
		},
		{
			Name:     "suffix_d",
			Args:     []string{"0d"},
			ExitCode: 0,
		},
		{
			Name:     "multiple_args_summed",
			Args:     []string{"0", "0.001", "0"},
			ExitCode: 0,
		},
		{
			Name:      "no_args_error",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrClear},
		},
		{
			Name:      "invalid_arg_error",
			Args:      []string{"abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrClear},
		},
		{
			Name:      "negative_arg_error",
			Args:      []string{"-1"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrClear},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
