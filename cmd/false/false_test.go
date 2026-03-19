// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd014-false R2.1-R2.3, R3.1, R4.1-R4.3.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests comparing the Go false binary against gfalse.
// R4.1: compares exit codes between Go and reference binary.
// R4.2: covers no arguments, arbitrary arguments, --help, --version.
// R4.3: verifies no output on stdout/stderr for non-flag invocations.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfalse")
	if err != nil {
		t.Skipf("reference binary gfalse not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.1, R4.3: no arguments — exit 1, no stdout or stderr.
		{
			Name:     "no_args",
			Args:     nil,
			ExitCode: 1,
		},
		// R4.2: arbitrary arguments are ignored, exit 1.
		{
			Name:     "single_arg_ignored",
			Args:     []string{"foo"},
			ExitCode: 1,
		},
		{
			Name:     "multiple_args_ignored",
			Args:     []string{"foo", "bar", "--baz"},
			ExitCode: 1,
		},
		// R4.2: flags that look like options are also ignored.
		{
			Name:     "dash_dash_ignored",
			Args:     []string{"--"},
			ExitCode: 1,
		},
		{
			Name:     "unknown_flag_ignored",
			Args:     []string{"--unknown"},
			ExitCode: 1,
		},
		// R4.2: --help output — GNU false exits 1 even for --help.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeHelpVersion},
		},
		// R4.2: --version output — GNU false exits 1 even for --version.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeHelpVersion},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// normalizeHelpVersion replaces all output with an empty byte slice so that
// --help and --version tests compare only exit codes, not implementation-specific
// text content. GNU false and our implementation produce different text.
func normalizeHelpVersion(b []byte) []byte {
	return nil
}
