// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsleep")
	if err != nil {
		t.Skipf("reference binary gsleep not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1, R2.1: zero duration exits immediately with 0.
		{
			Name:     "zero_duration",
			Args:     []string{"0"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: single numeric argument.
		{
			Name:     "short_duration",
			Args:     []string{"0.01"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.2: fractional seconds.
		{
			Name:     "fractional_seconds",
			Args:     []string{"0.001"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.3: suffix s (seconds).
		{
			Name:     "suffix_s",
			Args:     []string{"0.001s"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.3: suffix m (minutes).
		{
			Name:     "suffix_m",
			Args:     []string{"0.0001m"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.3: suffix h (hours).
		{
			Name:     "suffix_h",
			Args:     []string{"0.000001h"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.3: suffix d (days).
		{
			Name:     "suffix_d",
			Args:     []string{"0.0000001d"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.4: multiple arguments summed.
		{
			Name:     "multiple_args_summed",
			Args:     []string{"0", "0.001", "0.001s"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.2: no arguments → error (discard stderr, binary names differ).
		{
			Name:      "no_args_error",
			Args:      []string{},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R2.3: invalid argument → error.
		{
			Name:      "invalid_arg_error",
			Args:      []string{"abc"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R2.3: negative argument → error.
		{
			Name:      "negative_arg_error",
			Args:      []string{"-1"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
