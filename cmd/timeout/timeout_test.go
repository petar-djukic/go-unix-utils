// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for prd063-timeout R1.1-R1.4: core timeout behavior including
// command execution with time limit, fractional durations, suffix multipliers,
// and zero-duration bypass.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtimeout")
	if err != nil {
		t.Skipf("reference binary gtimeout not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		{
			// R1.1: command completes before timeout, exit with command status 0.
			Name:     "command_completes_exit_0",
			Args:     []string{"10", "true"},
			ExitCode: 0,
		},
		{
			// R1.1: command completes before timeout, exit with command status 1.
			Name:     "command_completes_exit_1",
			Args:     []string{"10", "false"},
			ExitCode: 1,
		},
		{
			// R1.1: command exceeds timeout, killed with SIGTERM, exit 124.
			Name:     "command_exceeds_timeout",
			Args:     []string{"0.01", "sleep", "10"},
			ExitCode: 124,
		},
		{
			// R1.2: fractional duration with fast command.
			Name:     "fractional_duration",
			Args:     []string{"0.5", "true"},
			ExitCode: 0,
		},
		{
			// R1.4: duration 0 disables the timeout, command runs normally.
			Name:     "duration_zero_no_limit",
			Args:     []string{"0", "true"},
			ExitCode: 0,
		},
		{
			// R1.3: suffix 's' for seconds.
			Name:     "suffix_seconds",
			Args:     []string{"10s", "true"},
			ExitCode: 0,
		},
		{
			// R1.3: suffix 'm' for minutes.
			Name:     "suffix_minutes",
			Args:     []string{"1m", "true"},
			ExitCode: 0,
		},
		{
			// R1.3: suffix 'h' for hours.
			Name:     "suffix_hours",
			Args:     []string{"1h", "true"},
			ExitCode: 0,
		},
		{
			// R1.3: suffix 'd' for days.
			Name:     "suffix_days",
			Args:     []string{"1d", "true"},
			ExitCode: 0,
		},
		{
			// R1.2 + R1.3: fractional duration with suffix causes timeout.
			Name:     "fractional_suffix_timeout",
			Args:     []string{"0.01s", "sleep", "10"},
			ExitCode: 124,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
