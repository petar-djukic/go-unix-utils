// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd049-realpath R4.1–R4.3 (differential tests for R1.1–R1.5, R2.1–R2.3, R3.1–R3.3)
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

// TestDiffStrip tests R1.5: -s / --strip / --no-symlinks mode.
func TestDiffStrip(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R1.5: -s with /tmp (symlink on macOS) should NOT resolve symlinks.
		{
			Name: "strip_short_flag",
			Args: []string{"-s", "/tmp"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.5: --strip long form.
		{
			Name: "strip_long_flag",
			Args: []string{"--strip", "/tmp"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.5: --no-symlinks alias.
		{
			Name: "no_symlinks_flag",
			Args: []string{"--no-symlinks", "/tmp"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.5: -s cleans .. components — intermediate component must exist.
		{
			Name:     "strip_dotdot_nonexistent",
			Args:     []string{"-s", "/tmp/foo/.."},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R1.5: -s cleans .. with existing intermediate.
		{
			Name: "strip_dotdot_existing",
			Args: []string{"-s", "/tmp/.."},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.5: -s cleans . components.
		{
			Name: "strip_dot",
			Args: []string{"-s", "/tmp/."},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.5: -s with nonexistent last component (parent exists).
		{
			Name: "strip_nonexistent_last",
			Args: []string{"-s", "/tmp/nonexistent_xyz_12345"},
			Env:  []string{"LC_ALL=C"},
		},
	}

	// Normalize stderr for error cases since messages may differ.
	clearStderr := func(b []byte) []byte { return nil }
	for i := range tests {
		if tests[i].ExitCode != 0 {
			tests[i].Normalize = []testutils.NormalizeFunc{clearStderr}
		}
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffRelativeTo tests R2.1: --relative-to output.
func TestDiffRelativeTo(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R2.1: print /tmp relative to /.
		{
			Name: "relative_to_root",
			Args: []string{"--relative-to=/", "/tmp"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: print / relative to /tmp.
		{
			Name: "relative_to_tmp",
			Args: []string{"--relative-to=/tmp", "/"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: same directory yields ".".
		{
			Name: "relative_to_same",
			Args: []string{"--relative-to=/tmp", "/tmp"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: combined with -s to avoid symlink interference.
		{
			Name: "relative_to_with_strip",
			Args: []string{"-s", "--relative-to=/", "/tmp"},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffRelativeBase tests R2.2: --relative-base output.
func TestDiffRelativeBase(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R2.2: path under base → relative output.
		{
			Name: "base_under",
			Args: []string{"-s", "--relative-base=/", "/tmp"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: path equals base → ".".
		{
			Name: "base_equals",
			Args: []string{"-s", "--relative-base=/tmp", "/tmp"},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffRelativeBoth tests R2.3: --relative-to + --relative-base combined.
func TestDiffRelativeBoth(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R2.3: path starts with base → apply relative-to.
		{
			Name: "both_under_base",
			Args: []string{"-s", "--relative-to=/", "--relative-base=/", "/tmp"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: path does NOT start with base → print absolute.
		{
			Name: "both_outside_base",
			Args: []string{"-s", "--relative-to=/var", "--relative-base=/var", "/tmp"},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffErrorNoOperand tests R3.1: no operand produces usage error, exit 1.
func TestDiffErrorNoOperand(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R3.1: no arguments at all.
		{
			Name:     "no_args",
			Args:     []string{},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R3.1: only flags, no operands.
		{
			Name:     "flags_only_no_operand",
			Args:     []string{"-s"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
	}

	// Error messages differ between implementations; normalize stderr.
	clearStderr := func(b []byte) []byte { return nil }
	for i := range tests {
		tests[i].Normalize = []testutils.NormalizeFunc{clearStderr}
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffErrorUnknownFlag tests R3.2: unknown flags produce error, exit 1.
func TestDiffErrorUnknownFlag(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R3.2: unknown short flag.
		{
			Name:     "unknown_short_flag",
			Args:     []string{"-j", "/tmp"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R3.2: unknown long flag.
		{
			Name:     "unknown_long_flag",
			Args:     []string{"--bogus", "/tmp"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
	}

	// Error messages differ between implementations; normalize stderr.
	clearStderr := func(b []byte) []byte { return nil }
	for i := range tests {
		tests[i].Normalize = []testutils.NormalizeFunc{clearStderr}
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffErrorMixedPaths tests R3.3: multiple paths where some fail still
// print successful resolutions and exit 1.
func TestDiffErrorMixedPaths(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R3.3: first path succeeds, second fails (parent does not exist).
		{
			Name:     "first_ok_second_fails",
			Args:     []string{"/tmp", "/nonexistent_parent_xyz/child"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R3.3: first fails, second succeeds.
		{
			Name:     "first_fails_second_ok",
			Args:     []string{"/nonexistent_parent_xyz/child", "/tmp"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R3.3: multiple failures.
		{
			Name:     "both_fail",
			Args:     []string{"/nonexistent_parent_xyz/a", "/nonexistent_parent_xyz/b"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
	}

	// Error messages differ between implementations; normalize stderr.
	clearStderr := func(b []byte) []byte { return nil }
	for i := range tests {
		tests[i].Normalize = []testutils.NormalizeFunc{clearStderr}
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
