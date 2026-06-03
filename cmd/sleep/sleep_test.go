// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"math"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsleep")
	if err != nil {
		t.Skip("reference binary not found")
	}

	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })
	discardStdout := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	tests := []testutils.DiffTest{
		// R3.3: --help prints usage to stdout, exit 0
		{Name: "help", Args: []string{"--help"}, Normalize: []testutils.NormalizeFunc{discardStdout}},
		// R3.4: --version prints version info to stdout, exit 0
		{Name: "version", Args: []string{"--version"}, Normalize: []testutils.NormalizeFunc{discardStdout}},
		// R1.1, R2.1, R4.1, R4.4: zero duration returns immediately with exit 0
		{Name: "zero", Args: []string{"0"}},
		// R1.1, R1.2, R4.1, R4.4: fractional seconds
		{Name: "fractional", Args: []string{"0.01"}},
		// R1.3, R4.4: suffix s (seconds)
		{Name: "suffix-s", Args: []string{"0.01s"}},
		// R1.3, R4.4: suffix m (minutes)
		{Name: "suffix-m", Args: []string{"0m"}},
		// R1.3, R4.4: suffix h (hours)
		{Name: "suffix-h", Args: []string{"0h"}},
		// R1.3, R4.4: suffix d (days)
		{Name: "suffix-d", Args: []string{"0d"}},
		// R1.4, R4.4: multiple arguments summed
		{Name: "multiple-args", Args: []string{"0", "0"}},
		// R1.4, R4.4: multiple with fractional
		{Name: "multiple-fractional", Args: []string{"0.01", "0.01"}},
		// R1.4, R4.4: multiple with suffixes
		{Name: "multiple-suffixes", Args: []string{"0s", "0m"}},
		// R2.2, R4.2, R4.4: no arguments → error
		{Name: "no-args", ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardStderr}},
		// R2.3, R4.2, R4.4: non-numeric argument → error
		{Name: "invalid-arg", Args: []string{"abc"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardStderr}},
		// R2.3, R4.2, R4.4: negative argument → error
		{Name: "negative-arg", Args: []string{"-1"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardStderr}},
		// R2.3, R4.2: empty string argument → error
		{Name: "empty-string", Args: []string{""}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardStderr}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// R2.4: infinity/inf cannot be differentially tested (both binaries block
// forever), so verify parseDuration directly.
func TestParseDurationInfinity(t *testing.T) {
	for _, arg := range []string{"inf", "infinity", "Inf", "INF", "Infinity", "INFINITY"} {
		val, err := parseDuration(arg)
		if err != nil {
			t.Errorf("parseDuration(%q) returned error: %v", arg, err)
		}
		if !math.IsInf(val, 1) {
			t.Errorf("parseDuration(%q) = %v, want +Inf", arg, val)
		}
	}
}
