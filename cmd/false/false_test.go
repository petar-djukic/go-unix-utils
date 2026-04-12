// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/false. Implements srd014-false R4.1, R4.2, R4.3.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// clearOutput returns nil to ignore stdout content differences.
// Used for --help and --version where output text differs between Go and GNU.
func clearOutput(b []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfalse")
	if err != nil {
		t.Skip("reference binary gfalse not in PATH")
	}

	tests := []testutils.DiffTest{
		{
			// R4.2, R4.3: no arguments — exit 1, no output.
			Name:     "no_args",
			Args:     []string{},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		{
			// R4.2: arbitrary arguments are ignored, exit 1.
			Name:     "ignored_args",
			Args:     []string{"foo", "bar", "--baz"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		{
			// R4.2, R4.3: single arbitrary argument, no output, exit 1.
			Name:     "single_arg",
			Args:     []string{"hello"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		{
			// R4.2: --help prints usage to stdout, still exits 1.
			// Normalize stdout because GNU help includes binary path and ANSI formatting.
			Name:      "help_flag",
			Args:      []string{"--help"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			// R4.2: --version prints version info to stdout, still exits 1.
			// Normalize stdout because version strings differ between Go and GNU.
			Name:      "version_flag",
			Args:      []string{"--version"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			// R3.1, R3.2: --version with trailing args still exits 1.
			Name:      "version_with_extra_args",
			Args:      []string{"--version", "extra"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			// R4.3: unrecognized double-dash flags still exit 1 with no output.
			Name:     "unrecognized_double_dash_flag",
			Args:     []string{"--unknown"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
