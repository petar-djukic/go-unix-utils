// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/true. Implements srd013-true R4.1, R4.2, R4.3.
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
		},
		{
			// R4.2: arbitrary arguments are ignored, exit 0.
			Name: "ignored_args",
			Args: []string{"foo", "bar", "--baz"},
		},
		{
			// R4.2, R4.3: single arbitrary argument, no output.
			Name: "single_arg",
			Args: []string{"hello"},
		},
		{
			// R4.2: --help prints usage to stdout, exit 0.
			// Normalize stdout because GNU help includes binary path and ANSI formatting.
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
