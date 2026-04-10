// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff verifies timeout exit code and output parity against gtimeout
// (GNU coreutils). Uses fast commands (true, false, sleep 0) and short
// durations to avoid slow test execution (R4.3).
// R1.1: run command and kill on timeout.
// R1.2: fractional durations.
// R1.3: suffix multipliers s, m, h, d.
// R1.4: duration 0 = no time limit.
// R2.1: -s SIGNAL / --signal=SIGNAL for signal selection.
// R2.2: -k DURATION / --kill-after=DURATION for kill escalation.
// R2.3: --foreground skips process group creation.
// R2.4: --preserve-status returns command exit status on timeout.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtimeout")
	if err != nil {
		t.Skip("reference binary gtimeout not in PATH")
	}
	stderrNorm := makeBinaryNameNormalizer(refBin)
	errNorms := []testutils.NormalizeFunc{stderrNorm}

	tests := []testutils.DiffTest{
		// R1.1: command succeeds before timeout.
		{
			Name: "cmd-succeeds",
			Args: []string{"10", "true"},
		},
		// R1.1: command fails before timeout, exit with command status.
		{
			Name:     "cmd-fails",
			Args:     []string{"10", "false"},
			ExitCode: 1,
		},
		// R1.1: command killed on timeout, exit 124.
		{
			Name:     "cmd-timeout",
			Args:     []string{"0.1", "sleep", "10"},
			ExitCode: 124,
		},
		// R1.2: fractional duration, command finishes before timeout.
		{
			Name: "fractional-duration",
			Args: []string{"0.5", "true"},
		},
		// R1.3: suffix s.
		{
			Name: "suffix-s",
			Args: []string{"10s", "true"},
		},
		// R1.3: suffix m.
		{
			Name: "suffix-m",
			Args: []string{"1m", "true"},
		},
		// R1.3: suffix h.
		{
			Name: "suffix-h",
			Args: []string{"1h", "true"},
		},
		// R1.3: suffix d.
		{
			Name: "suffix-d",
			Args: []string{"1d", "true"},
		},
		// R1.3: fractional with suffix, triggers actual timeout.
		{
			Name:     "suffix-s-timeout",
			Args:     []string{"0.1s", "sleep", "10"},
			ExitCode: 124,
		},
		// R1.4: duration 0, no time limit.
		{
			Name: "zero-no-limit",
			Args: []string{"0", "true"},
		},
		// R1.4: duration 0 with command that takes a moment.
		{
			Name: "zero-no-limit-sleep",
			Args: []string{"0", "sleep", "0"},
		},
		// R1.4: duration 0 with suffix.
		{
			Name: "zero-suffix-s",
			Args: []string{"0s", "true"},
		},
		// R2.1: -s HUP sends SIGHUP on timeout.
		{
			Name:     "signal-hup-timeout",
			Args:     []string{"-s", "HUP", "0.1", "sleep", "10"},
			ExitCode: 124,
		},
		// R2.1: -s with numeric signal value (1 = SIGHUP).
		{
			Name:     "signal-numeric-1",
			Args:     []string{"-s", "1", "0.1", "sleep", "10"},
			ExitCode: 124,
		},
		// R2.1: --signal=HUP long form.
		{
			Name:     "signal-long-form",
			Args:     []string{"--signal=HUP", "0.1", "sleep", "10"},
			ExitCode: 124,
		},
		// R2.1: -s TERM explicit, command succeeds before timeout.
		{
			Name: "signal-term-success",
			Args: []string{"-s", "TERM", "10", "true"},
		},
		// R2.2: -k kill-after with timeout.
		{
			Name:     "kill-after",
			Args:     []string{"-k", "0.5", "0.1", "sleep", "10"},
			ExitCode: 124,
		},
		// R2.2: --kill-after= long form.
		{
			Name:     "kill-after-long",
			Args:     []string{"--kill-after=0.5", "0.1", "sleep", "10"},
			ExitCode: 124,
		},
		// R2.3: --foreground with command that succeeds.
		{
			Name: "foreground-success",
			Args: []string{"--foreground", "10", "true"},
		},
		// R2.3: --foreground with timeout.
		{
			Name:     "foreground-timeout",
			Args:     []string{"--foreground", "0.1", "sleep", "10"},
			ExitCode: 124,
		},
		// R2.4: --preserve-status with timeout returns 128+SIGTERM(15)=143.
		{
			Name:     "preserve-status-timeout",
			Args:     []string{"--preserve-status", "0.1", "sleep", "10"},
			ExitCode: 143,
		},
		// R2.4: --preserve-status without timeout, exits with command status.
		{
			Name: "preserve-status-success",
			Args: []string{"--preserve-status", "10", "true"},
		},
		// R2.4: --preserve-status with failing command, no timeout.
		{
			Name:     "preserve-status-failure",
			Args:     []string{"--preserve-status", "10", "false"},
			ExitCode: 1,
		},
		// R2.1+R2.4: --preserve-status -s HUP returns 128+SIGHUP(1)=129.
		{
			Name:     "preserve-status-signal-hup",
			Args:     []string{"--preserve-status", "-s", "HUP", "0.1", "sleep", "10"},
			ExitCode: 129,
		},
		// Error: no arguments.
		{
			Name:      "no-args",
			Args:      []string{},
			ExitCode:  exitInternal,
			Normalize: errNorms,
		},
		// Error: missing command (only duration provided).
		{
			Name:      "missing-cmd",
			Args:      []string{"10"},
			ExitCode:  exitInternal,
			Normalize: errNorms,
		},
		// Error: invalid duration.
		{
			Name:      "invalid-duration",
			Args:      []string{"abc", "true"},
			ExitCode:  exitInternal,
			Normalize: errNorms,
		},
		// Error: command not found.
		{
			Name:      "cmd-not-found",
			Args:      []string{"10", "nonexistent_cmd_xyz_12345"},
			ExitCode:  exitNotFound,
			Normalize: errNorms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// makeBinaryNameNormalizer returns a NormalizeFunc that replaces the reference
// binary path and base name with progName so stderr messages match.
func makeBinaryNameNormalizer(refBin string) testutils.NormalizeFunc {
	refBase := filepath.Base(refBin)
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte(progName))
		b = bytes.ReplaceAll(b, []byte(refBase), []byte(progName))
		return b
	}
}
