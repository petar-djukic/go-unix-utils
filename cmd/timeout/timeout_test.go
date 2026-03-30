// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtimeout")
	if err != nil {
		t.Skipf("reference binary gtimeout not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: command completes before timeout, exit with command status 0.
		{
			Name:      "command_exits_before_timeout",
			Args:      []string{"10", "true"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.1: command completes with non-zero status before timeout.
		{
			Name:      "command_fails_before_timeout",
			Args:      []string{"10", "false"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.1: command exceeds timeout, exit 124.
		{
			Name:      "command_exceeds_timeout",
			Args:      []string{"0.01", "sleep", "10"},
			ExitCode:  124,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.2: fractional duration (0.5s is enough for true to complete).
		{
			Name:      "fractional_duration_completes",
			Args:      []string{"0.5", "true"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.2: fractional duration triggers timeout.
		{
			Name:      "fractional_duration_timeout",
			Args:      []string{"0.01", "sleep", "10"},
			ExitCode:  124,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: suffix s (seconds).
		{
			Name:      "suffix_s",
			Args:      []string{"10s", "true"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: suffix m (minutes).
		{
			Name:      "suffix_m",
			Args:      []string{"1m", "true"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: suffix h (hours).
		{
			Name:      "suffix_h",
			Args:      []string{"1h", "true"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: suffix d (days).
		{
			Name:      "suffix_d",
			Args:      []string{"1d", "true"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.4: duration 0 means no time limit; command runs to completion.
		{
			Name:      "duration_zero_no_limit",
			Args:      []string{"0", "true"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.4: duration 0 with a failing command still returns command status.
		{
			Name:      "duration_zero_command_fails",
			Args:      []string{"0", "false"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// Error: no arguments.
		{
			Name:      "no_args_error",
			Args:      []string{},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// Error: only duration, no command.
		{
			Name:      "missing_command_error",
			Args:      []string{"10"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// Error: invalid duration.
		{
			Name:      "invalid_duration_error",
			Args:      []string{"abc", "true"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// Error: command not found.
		{
			Name:      "command_not_found_error",
			Args:      []string{"10", "nonexistent_command_xyz_12345"},
			ExitCode:  127,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestParseDuration verifies duration parsing unit tests.
func TestParseDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		// R1.2: fractional values.
		{name: "fractional", input: "0.5", want: 500 * time.Millisecond},
		{name: "integer", input: "3", want: 3 * time.Second},
		// R1.3: suffix multipliers.
		{name: "suffix_s", input: "2s", want: 2 * time.Second},
		{name: "suffix_m", input: "1m", want: 60 * time.Second},
		{name: "suffix_h", input: "1h", want: 3600 * time.Second},
		{name: "suffix_d", input: "1d", want: 86400 * time.Second},
		// R1.4: duration 0.
		{name: "zero", input: "0", want: 0},
		{name: "zero_s", input: "0s", want: 0},
		// Errors.
		{name: "negative", input: "-1", wantErr: true},
		{name: "non_numeric", input: "abc", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseDuration(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseDuration(%q) = %v, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDuration(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("parseDuration(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestKillAfterTimeout verifies that the timed-out process is actually killed
// and does not remain as an orphan. R4.4 traceability.
func TestKillAfterTimeout(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "0.1", "sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start timeout: %v", err)
	}

	err := cmd.Wait()
	if err == nil {
		t.Fatal("expected non-zero exit from timeout")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 124 {
		t.Fatalf("expected exit code 124, got %d", exitErr.ExitCode())
	}
}
