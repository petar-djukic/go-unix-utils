// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var clearStderr testutils.NormalizeFunc = func(b []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtimeout")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		{
			Name:     "command_completes_before_timeout",
			Args:     []string{"10", "true"},
			ExitCode: 0,
		},
		{
			Name:     "command_fails_before_timeout",
			Args:     []string{"10", "false"},
			ExitCode: 1,
		},
		{
			Name:     "command_exceeds_timeout",
			Args:     []string{"0.01", "sleep", "10"},
			ExitCode: 124,
		},
		{
			Name:     "fractional_duration",
			Args:     []string{"0.5", "true"},
			ExitCode: 0,
		},
		{
			Name:     "suffix_seconds",
			Args:     []string{"10s", "true"},
			ExitCode: 0,
		},
		{
			Name:     "suffix_minutes",
			Args:     []string{"1m", "true"},
			ExitCode: 0,
		},
		{
			Name:     "duration_zero_no_limit",
			Args:     []string{"0", "true"},
			ExitCode: 0,
		},
		{
			Name:     "duration_zero_suffix_no_limit",
			Args:     []string{"0s", "true"},
			ExitCode: 0,
		},
		{
			Name:      "no_args",
			Args:      []string{},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		{
			Name:      "missing_command",
			Args:      []string{"10"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		{
			Name:      "command_not_found",
			Args:      []string{"10", "nonexistent_command_xyz_123"},
			ExitCode:  127,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		{
			Name:      "invalid_duration",
			Args:      []string{"abc", "true"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
