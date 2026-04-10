// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff verifies sleep exit code parity against gsleep (GNU coreutils).
// Uses short durations (0 or 0.01) to avoid slow test execution.
// R1.1: single numeric argument. R1.2: fractional seconds.
// R1.3: suffix multipliers. R1.4: multiple arguments summed.
// R2.1: zero duration. R2.2: no args error. R2.3: invalid/negative error.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsleep")
	if err != nil {
		t.Skip("reference binary gsleep not in PATH")
	}
	stderrNorm := makeBinaryNameNormalizer(refBin)
	errNorms := []testutils.NormalizeFunc{stderrNorm}

	tests := []testutils.DiffTest{
		// R1.1: zero duration exits immediately with 0.
		{
			Name: "zero-duration",
			Args: []string{"0"},
		},
		// R1.2: fractional seconds.
		{
			Name: "fractional-seconds",
			Args: []string{"0.01"},
		},
		// R1.3: suffix multipliers.
		{
			Name: "suffix-s",
			Args: []string{"0.01s"},
		},
		{
			Name: "suffix-m",
			Args: []string{"0.001m"},
		},
		{
			Name: "suffix-h",
			Args: []string{"0.00001h"},
		},
		{
			Name: "suffix-d",
			Args: []string{"0.000001d"},
		},
		// R1.4: multiple arguments summed.
		{
			Name: "multiple-args-summed",
			Args: []string{"0", "0.01", "0"},
		},
		{
			Name: "multiple-args-with-suffix",
			Args: []string{"0.005s", "0.005s"},
		},
		// R2.1: zero is a valid duration, returns immediately with exit 0.
		{
			Name: "zero-explicit",
			Args: []string{"0.0"},
		},
		{
			Name: "zero-with-suffix",
			Args: []string{"0s"},
		},
		// R2.2: no arguments gives usage error, exit 1.
		{
			Name:      "no-args",
			Args:      []string{},
			ExitCode:  1,
			Normalize: errNorms,
		},
		// R2.3: non-numeric or negative argument gives error, exit 1.
		{
			Name:      "invalid-arg",
			Args:      []string{"abc"},
			ExitCode:  1,
			Normalize: errNorms,
		},
		{
			Name:      "negative-arg",
			Args:      []string{"-1"},
			ExitCode:  1,
			Normalize: errNorms,
		},
		{
			Name:      "invalid-empty-suffix",
			Args:      []string{"s"},
			ExitCode:  1,
			Normalize: errNorms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestInfinity verifies that sleep accepts inf and infinity as valid
// durations (R2.4). Since these sleep indefinitely, we start the process,
// verify it does not exit with an error within a short window, then kill it.
func TestInfinity(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	cases := []struct {
		name string
		arg  string
	}{
		{"inf", "inf"},
		{"infinity", "infinity"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertSleepsIndefinitely(t, goBin, tc.arg)
		})
	}
}

// assertSleepsIndefinitely starts the binary with the given arg, waits
// briefly, and verifies it is still running (not exited with an error).
func assertSleepsIndefinitely(t *testing.T, bin, arg string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, arg)
	err := cmd.Run()
	// The context should have killed it; a deadline-exceeded error is expected.
	if ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("expected process to still be running after timeout, got err: %v", err)
	}
}

// makeBinaryNameNormalizer returns a NormalizeFunc that replaces the reference
// binary path with "sleep" so stderr error messages match between gsleep and
// our binary.
func makeBinaryNameNormalizer(refBin string) testutils.NormalizeFunc {
	refBase := filepath.Base(refBin)
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte(progName))
		b = bytes.ReplaceAll(b, []byte(refBase), []byte(progName))
		return b
	}
}
