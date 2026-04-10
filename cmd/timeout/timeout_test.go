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
