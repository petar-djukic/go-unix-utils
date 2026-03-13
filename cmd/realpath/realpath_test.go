// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd049-realpath R4.1–R4.3 (differential tests for R1.1–R1.4)
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for realpath.
const refBinaryName = "grealpath"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R1.1: resolve a simple existing path.
		{
			Name: "resolve_tmp",
			Args: []string{"/tmp"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: resolve root.
		{
			Name: "resolve_root",
			Args: []string{"/"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: resolve dot (current directory).
		{
			Name: "resolve_dot",
			Args: []string{"."},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: multiple path operands, all valid.
		{
			Name: "multiple_paths",
			Args: []string{"/tmp", "/"},
			Env:  []string{"LC_ALL=C"},
		},
		// Default mode: last component may not exist (parent / exists).
		{
			Name: "nonexistent_last_component",
			Args: []string{"/nonexistent_path_xyz_12345"},
			Env:  []string{"LC_ALL=C"},
		},
		// Default mode: multiple paths, last components may not exist.
		{
			Name: "mixed_existing_nonexistent",
			Args: []string{"/tmp", "/nonexistent_path_xyz_12345"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: parent does not exist — error, exit 1.
		{
			Name:     "nonexistent_parent",
			Args:     []string{"/nonexistent_parent_xyz/child"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R1.1: path with .. component.
		{
			Name: "dotdot_component",
			Args: []string{"/tmp/.."},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: path with . component.
		{
			Name: "dot_component",
			Args: []string{"/tmp/."},
			Env:  []string{"LC_ALL=C"},
		},
	}

	// Normalize stderr since error messages may differ in formatting.
	clearStderr := func(b []byte) []byte { return nil }
	for i := range tests {
		if tests[i].ExitCode != 0 {
			tests[i].Normalize = []testutils.NormalizeFunc{clearStderr}
		}
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffHelpVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R1.4: --help exits 0.
		{
			Name: "help_flag",
			Args: []string{"--help"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: --version exits 0.
		{
			Name: "version_flag",
			Args: []string{"--version"},
			Env:  []string{"LC_ALL=C"},
		},
	}

	// --help and --version produce different output between implementations,
	// so we only compare exit codes by normalizing stdout/stderr to empty.
	clearOutput := func(b []byte) []byte { return nil }
	for i := range tests {
		tests[i].Normalize = []testutils.NormalizeFunc{clearOutput}
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffErrors(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R3.1: no arguments.
		{
			Name:     "no_args",
			Args:     []string{},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
	}

	// Error messages differ between implementations; normalize stderr.
	clearOutput := func(b []byte) []byte { return nil }
	for i := range tests {
		tests[i].Normalize = []testutils.NormalizeFunc{clearOutput}
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
