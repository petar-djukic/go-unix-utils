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

// TestDiff verifies sleep exit code parity against gsleep (GNU coreutils).
// Uses short durations (0 or 0.01) to avoid slow test execution.
// R1.1: single numeric argument. R1.2: fractional seconds.
// R1.3: suffix multipliers. R1.4: multiple arguments summed.
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
		// Error cases for differential parity.
		{
			Name:      "no-args",
			Args:      []string{},
			ExitCode:  1,
			Normalize: errNorms,
		},
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
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
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
