// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests: prd014-false R1.1–R1.3, R2.1–R2.2, R3.1–R3.2, R4.1–R4.3 via differential
// testing against gfalse (Homebrew GNU false).
package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfalse")
	if err != nil {
		t.Skipf("reference binary gfalse not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1, R4.3: no arguments, exits 1, no output.
		{
			Name:     "false_no_args",
			ExitCode: 1,
		},
		// R1.2: arbitrary arguments ignored, exits 1.
		{
			Name:     "false_with_args",
			Args:     []string{"foo", "bar", "baz"},
			ExitCode: 1,
		},
		// R1.2: unknown flags ignored, exits 1.
		{
			Name:     "false_unknown_flags",
			Args:     []string{"--unknown", "-x", "-abc"},
			ExitCode: 1,
		},
		// R2.1: --help prints usage to stdout; GNU false still exits 1.
		{
			Name:      "false_help",
			Args:      []string{"--help"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeHelp},
		},
		// R2.2: --version prints version info to stdout; GNU false still exits 1.
		{
			Name:      "false_version",
			Args:      []string{"--version"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeVersion},
		},
		// R1.2: single dash is not special, ignored like any argument.
		{
			Name:     "false_single_dash",
			Args:     []string{"-"},
			ExitCode: 1,
		},
		// R1.2: double dash ignored.
		{
			Name:     "false_double_dash",
			Args:     []string{"--"},
			ExitCode: 1,
		},
		// R1.2: --help not first argument; GNU false ignores it.
		{
			Name:     "false_help_not_first",
			Args:     []string{"foo", "--help"},
			ExitCode: 1,
		},
		// R1.2: --version not first argument; GNU false ignores it.
		{
			Name:     "false_version_not_first",
			Args:     []string{"foo", "--version"},
			ExitCode: 1,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// normalizeHelp discards the full help text so only exit code is compared.
// GNU false's --help output includes paths and details that differ from our implementation.
func normalizeHelp(data []byte) []byte {
	if len(data) > 0 {
		return []byte("HELP_OUTPUT\n")
	}
	return data
}

// normalizeVersion replaces version-specific text so only the presence of output
// and exit code are compared. GNU false prints "false (GNU coreutils) X.Y" while
// our binary prints "false dev".
func normalizeVersion(data []byte) []byte {
	if bytes.Contains(data, []byte("false")) {
		return []byte("VERSION_OUTPUT\n")
	}
	return data
}
