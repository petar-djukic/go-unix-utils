// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"math"
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
		// R3.1: no output to stdout or stderr under normal operation.
		{
			Name:     "no_output_normal",
			Args:     []string{"0.001"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.2: must not read from stdin (stdin provided but ignored).
		{
			Name:     "stdin_ignored",
			Args:     []string{"0"},
			Stdin:    []byte("this input should be ignored\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.3: --help prints usage to stdout and exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R3.4: --version prints version info to stdout and exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestParseDuration verifies duration parsing for cases that cannot be tested
// via differential testing (e.g., infinity would hang both binaries).
func TestParseDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		arg     string
		wantInf bool
		wantErr bool
		wantSec float64
	}{
		// R2.1: zero duration.
		{name: "zero", arg: "0", wantSec: 0},
		// R2.4: infinity variants.
		{name: "inf", arg: "inf", wantInf: true},
		{name: "infinity", arg: "infinity", wantInf: true},
		{name: "Inf", arg: "Inf", wantInf: true},
		{name: "INF", arg: "INF", wantInf: true},
		{name: "INFINITY", arg: "INFINITY", wantInf: true},
		// R2.3: invalid arguments.
		{name: "non_numeric", arg: "abc", wantErr: true},
		{name: "negative", arg: "-1", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseDuration(tc.arg)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseDuration(%q) = %v, want error", tc.arg, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDuration(%q) unexpected error: %v", tc.arg, err)
			}
			if tc.wantInf {
				if !math.IsInf(got, 1) {
					t.Fatalf("parseDuration(%q) = %v, want +Inf", tc.arg, got)
				}
				return
			}
			if got != tc.wantSec {
				t.Fatalf("parseDuration(%q) = %v, want %v", tc.arg, got, tc.wantSec)
			}
		})
	}
}

// TestSumDurationsInfinity verifies that infinity arguments produce the
// maximum duration (R2.4). This cannot be tested via RunDiffTests because
// both binaries would sleep forever.
func TestSumDurationsInfinity(t *testing.T) {
	t.Parallel()

	dur, err := sumDurations([]string{"inf"})
	if err != nil {
		t.Fatalf("sumDurations([inf]) unexpected error: %v", err)
	}
	if dur != time.Duration(math.MaxInt64) {
		t.Fatalf("sumDurations([inf]) = %v, want max duration", dur)
	}

	dur, err = sumDurations([]string{"0.01", "infinity"})
	if err != nil {
		t.Fatalf("sumDurations([0.01, infinity]) unexpected error: %v", err)
	}
	if dur != time.Duration(math.MaxInt64) {
		t.Fatalf("sumDurations([0.01, infinity]) = %v, want max duration", dur)
	}
}
