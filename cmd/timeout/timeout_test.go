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
		// R2.1: signal selection with named signal (SIGKILL kills timeout too)
		{
			Name:     "signal_kill_named",
			Args:     []string{"-s", "KILL", "0.01", "sleep", "10"},
			ExitCode: -1,
		},
		{
			Name:     "signal_kill_long",
			Args:     []string{"--signal=KILL", "0.01", "sleep", "10"},
			ExitCode: -1,
		},
		// R2.1: signal selection with numeric signal
		{
			Name:     "signal_numeric_9",
			Args:     []string{"-s", "9", "0.01", "sleep", "10"},
			ExitCode: -1,
		},
		// R2.1: signal HUP
		{
			Name:     "signal_hup",
			Args:     []string{"-s", "HUP", "0.01", "sleep", "10"},
			ExitCode: 129,
		},
		// R2.2: kill-after escalation
		{
			Name:     "kill_after",
			Args:     []string{"-k", "0.01", "0.01", "sleep", "10"},
			ExitCode: 124,
		},
		{
			Name:     "kill_after_long",
			Args:     []string{"--kill-after=0.01", "0.01", "sleep", "10"},
			ExitCode: 124,
		},
		// R2.3: foreground mode
		{
			Name:     "foreground_completes",
			Args:     []string{"--foreground", "10", "true"},
			ExitCode: 0,
		},
		{
			Name:     "foreground_timeout",
			Args:     []string{"--foreground", "0.01", "sleep", "10"},
			ExitCode: 124,
		},
		// R2.4: preserve-status on timeout
		{
			Name:     "preserve_status_timeout",
			Args:     []string{"--preserve-status", "0.01", "sleep", "10"},
			ExitCode: 143,
		},
		// R2.4: preserve-status when command completes normally
		{
			Name:     "preserve_status_normal",
			Args:     []string{"--preserve-status", "10", "true"},
			ExitCode: 0,
		},
		{
			Name:     "preserve_status_failure",
			Args:     []string{"--preserve-status", "10", "false"},
			ExitCode: 1,
		},
		// combined: signal + preserve-status (SIGKILL kills timeout too)
		{
			Name:     "signal_kill_preserve_status",
			Args:     []string{"--preserve-status", "-s", "KILL", "0.01", "sleep", "10"},
			ExitCode: -1,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
