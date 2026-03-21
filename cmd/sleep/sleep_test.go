// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sleep against gsleep reference binary.
// Implements prd061-sleep R4.3, R4.4.
// Covers R2.1 (zero duration), R2.2 (no args error), R2.3 (invalid/negative args),
// R2.4 (infinity/inf support).
// Covers R3.1 (no output under normal op), R3.3 (--help), R3.4 (--version),
// and overflow/large value handling.
package main

import (
	"context"
	"os/exec"
	"testing"
	"time"

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
		// R2.1: zero duration returns immediately with exit 0.
		{
			Name:     "zero_duration",
			Args:     []string{"0"},
			ExitCode: 0,
		},
		// R2.1: zero with suffix.
		{
			Name:     "zero_duration_suffix_s",
			Args:     []string{"0s"},
			ExitCode: 0,
		},
		{
			Name:     "zero_duration_suffix_m",
			Args:     []string{"0m"},
			ExitCode: 0,
		},
		{
			Name:     "zero_duration_suffix_h",
			Args:     []string{"0h"},
			ExitCode: 0,
		},
		{
			Name:     "zero_duration_suffix_d",
			Args:     []string{"0d"},
			ExitCode: 0,
		},
		// R1.2: fractional seconds.
		{
			Name:     "fractional_seconds",
			Args:     []string{"0.001"},
			ExitCode: 0,
		},
		// R1.4: multiple arguments summed.
		{
			Name:     "multiple_args_summed",
			Args:     []string{"0", "0.001", "0"},
			ExitCode: 0,
		},
		// R1.4: multiple arguments with different suffixes.
		{
			Name:     "multiple_args_mixed_suffixes",
			Args:     []string{"0s", "0m", "0h"},
			ExitCode: 0,
		},
		// R2.2: no arguments error.
		{
			Name:      "no_args_error",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrClear},
		},
		// R2.3: non-numeric argument error.
		{
			Name:      "invalid_arg_error",
			Args:      []string{"abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrClear},
		},
		// R2.3: negative argument error.
		{
			Name:      "negative_arg_error",
			Args:      []string{"-1"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrClear},
		},
		// R2.3: negative fractional argument error.
		{
			Name:      "negative_fractional_error",
			Args:      []string{"-0.5"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrClear},
		},
		// R2.3: empty string argument error.
		{
			Name:      "empty_string_error",
			Args:      []string{""},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrClear},
		},
		// R3.1: normal operation produces no stdout or stderr.
		{
			Name:     "no_output_normal",
			Args:     []string{"0"},
			ExitCode: 0,
		},
		// R3.3: --help prints usage and exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{stderrClear},
		},
		// R3.4: --version prints version info and exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{stderrClear},
		},
		// R3.1: invalid suffix produces error.
		{
			Name:      "invalid_suffix_error",
			Args:      []string{"1x"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrClear},
		},
		// R2.3: multiple args with one invalid produces error.
		{
			Name:      "mixed_valid_invalid_error",
			Args:      []string{"0", "abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrClear},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestInfinity verifies that infinity and inf are accepted as valid durations
// (R2.4). These cannot be tested via differential testing because both binaries
// would sleep indefinitely. Instead, we start the binary, wait briefly to confirm
// it does not exit with an error, then cancel it.
func TestInfinity(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cases := []struct {
		name string
		arg  string
	}{
		{"infinity", "infinity"},
		{"inf", "inf"},
		{"inf_uppercase", "Inf"},
		{"infinity_mixed_case", "Infinity"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			verifyInfinityAccepted(t, goBin, tc.arg)
		})
	}
}

// TestOverflow verifies that extremely large duration values do not crash
// or produce errors. R3.4: overflow values are clamped and sleep indefinitely
// (like infinity). We verify by starting the binary and confirming it does not
// exit with an error within 100ms.
func TestOverflow(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cases := []struct {
		name string
		args []string
	}{
		{"huge_number", []string{"99999999999999999999999999"}},
		{"huge_with_suffix_d", []string{"99999999999999999999d"}},
		{"max_float", []string{"1.7976931348623157e+308"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			verifyLongSleepAccepted(t, goBin, tc.args)
		})
	}
}

// verifyLongSleepAccepted starts the binary with the given arguments and
// verifies it does not exit immediately with an error. Cancels after 100ms.
func verifyLongSleepAccepted(t *testing.T, binary string, args []string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	err := cmd.Run()

	if ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("sleep %v exited immediately (err=%v), expected to sleep", args, err)
	}
}

// verifyInfinityAccepted starts the binary with the given argument and
// verifies it does not exit immediately with an error. Cancels after 100ms.
func verifyInfinityAccepted(t *testing.T, binary, arg string) {
	t.Helper()
	verifyLongSleepAccepted(t, binary, []string{arg})
}
