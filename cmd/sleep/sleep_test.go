// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
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

	tests := []testutils.DiffTest{
		// R1.1, R2.1: zero duration returns immediately with exit 0
		{Name: "zero", Args: []string{"0"}},
		// R1.1, R1.2: fractional seconds
		{Name: "fractional", Args: []string{"0.01"}},
		// R1.3: suffix s (seconds)
		{Name: "suffix-s", Args: []string{"0.01s"}},
		// R1.3: suffix m (minutes)
		{Name: "suffix-m", Args: []string{"0m"}},
		// R1.3: suffix h (hours)
		{Name: "suffix-h", Args: []string{"0h"}},
		// R1.3: suffix d (days)
		{Name: "suffix-d", Args: []string{"0d"}},
		// R1.4: multiple arguments summed
		{Name: "multiple-args", Args: []string{"0", "0"}},
		// R1.4: multiple with fractional
		{Name: "multiple-fractional", Args: []string{"0.01", "0.01"}},
		// R1.4: multiple with suffixes
		{Name: "multiple-suffixes", Args: []string{"0s", "0m"}},
		// R2.2: no arguments → error
		{Name: "no-args", ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardStderr}},
		// R2.3: non-numeric argument → error
		{Name: "invalid-arg", Args: []string{"abc"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardStderr}},
		// R2.3: negative argument → error
		{Name: "negative-arg", Args: []string{"-1"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardStderr}},
		// R2.3: empty string argument → error
		{Name: "empty-string", Args: []string{""}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardStderr}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
