// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests: prd013-true R1.1–R1.3, R2.1–R2.2, R3.1–R3.2, R4.1–R4.3 via differential
// testing against gtrue (Homebrew GNU true).
package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtrue")
	if err != nil {
		t.Skipf("reference binary gtrue not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1, R4.3: no arguments, exits 0, no output.
		{
			Name: "true_no_args",
		},
		// R1.2: arbitrary arguments ignored, exits 0.
		{
			Name: "true_with_args",
			Args: []string{"foo", "bar", "baz"},
		},
		// R1.2: unknown flags ignored, exits 0.
		{
			Name: "true_unknown_flags",
			Args: []string{"--unknown", "-x", "-abc"},
		},
		// R2.1: --help prints usage to stdout, exits 0.
		{
			Name:      "true_help",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{normalizeHelp},
		},
		// R2.2: --version prints version info to stdout, exits 0.
		{
			Name:      "true_version",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{normalizeVersion},
		},
		// R1.2: single dash is not special, ignored like any argument.
		{
			Name: "true_single_dash",
			Args: []string{"-"},
		},
		// R1.2: double dash ignored.
		{
			Name: "true_double_dash",
			Args: []string{"--"},
		},
		// R1.2: --help not first argument; GNU true ignores it.
		{
			Name: "true_help_not_first",
			Args: []string{"foo", "--help"},
		},
		// R1.2: --version not first argument; GNU true ignores it.
		{
			Name: "true_version_not_first",
			Args: []string{"foo", "--version"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// normalizeHelp discards the full help text so only exit code is compared.
// GNU true's --help output includes paths and details that differ from our implementation.
func normalizeHelp(data []byte) []byte {
	if len(data) > 0 {
		return []byte("HELP_OUTPUT\n")
	}
	return data
}

// normalizeVersion replaces version-specific text so only the presence of output
// and exit code are compared. GNU true prints "true (GNU coreutils) X.Y" while
// our binary prints "true dev".
func normalizeVersion(data []byte) []byte {
	if bytes.Contains(data, []byte("true")) {
		return []byte("VERSION_OUTPUT\n")
	}
	return data
}
