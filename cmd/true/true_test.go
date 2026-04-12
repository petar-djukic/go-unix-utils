// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/true. Implements srd013-true R4.1, R4.2, R4.3 (R2.2, R2.3, R3.1, R3.2).
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// clearOutput returns a normalizer that replaces all output with empty bytes.
// Used for --help where stdout content differs but exit code must match.
func clearOutput(b []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtrue")
	if err != nil {
		t.Skip("reference binary gtrue not in PATH")
	}

	tests := []testutils.DiffTest{
		{
			// R4.2, R4.3: no arguments — exit 0, no output.
			Name: "no_args",
			Args: []string{},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R4.2: arbitrary arguments are ignored, exit 0.
			Name: "ignored_args",
			Args: []string{"foo", "bar", "--baz"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R4.2, R4.3: single arbitrary argument, no output.
			Name: "single_arg",
			Args: []string{"hello"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R4.2: --help prints usage to stdout, exit 0.
			// Normalize stdout because GNU help includes binary path and ANSI formatting.
			Name:      "help_flag",
			Args:      []string{"--help"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			// R2.2, R4.2: --version prints version info to stdout, exit 0.
			// Normalize stdout because version strings differ between Go and GNU.
			Name:      "version_flag",
			Args:      []string{"--version"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			// R3.1, R3.2: --version with trailing args still exits 0.
			// GNU true treats only the first arg for --version.
			Name:      "version_with_extra_args",
			Args:      []string{"--version", "extra"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			// R2.3, R3.1: unrecognized flags still exit 0 with no output.
			Name: "unrecognized_double_dash_flag",
			Args: []string{"--unknown"},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
