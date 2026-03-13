// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd051-pwd R3.1–R3.2 (differential tests for R1.1–R1.4, R2.1–R2.2)
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for pwd.
const refBinaryName = "gpwd"

// TestDiff tests R1.1–R1.4, R2.1–R2.2 via differential comparison with gpwd.
// R3.1: compares stdout and exit codes between the Go binary and gpwd.
// R3.2: covers default invocation, -L, -P, -L -P precedence, and error cases.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// Create a temporary directory with a symlink for -L testing.
	tmpDir := t.TempDir()

	// Create a real subdirectory.
	realDir := filepath.Join(tmpDir, "realdir")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("creating realdir: %v", err)
	}

	// Create a symlink pointing to the real directory.
	symlinkDir := filepath.Join(tmpDir, "linkdir")
	if err := os.Symlink(realDir, symlinkDir); err != nil {
		t.Fatalf("creating symlink dir: %v", err)
	}

	// Resolve the physical path of realDir for PWD env validation.
	physicalRealDir, err := filepath.EvalSymlinks(realDir)
	if err != nil {
		t.Fatalf("resolving realdir symlinks: %v", err)
	}

	// clearStderr normalizes stderr to nil so error message formatting
	// differences between binaries do not cause spurious failures.
	clearStderr := func(b []byte) []byte { return nil }

	tests := []testutils.DiffTest{
		// === R1.1: default invocation (no args) prints cwd and exits 0 ===
		{
			Name:    "default_no_args",
			Args:    []string{},
			Env:     []string{"LC_ALL=C"},
			WorkDir: physicalRealDir,
		},
		// === R1.3: -P prints physical path with symlinks resolved ===
		{
			Name:    "physical_flag",
			Args:    []string{"-P"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: physicalRealDir,
		},
		// === R1.3: --physical long form ===
		{
			Name:    "physical_long_form",
			Args:    []string{"--physical"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: physicalRealDir,
		},
		// === R1.2: -L prints the value of PWD when it is valid ===
		{
			Name:    "logical_flag_valid_pwd",
			Args:    []string{"-L"},
			Env:     []string{"LC_ALL=C", "PWD=" + symlinkDir},
			WorkDir: physicalRealDir,
		},
		// === R1.2: --logical long form ===
		{
			Name:    "logical_long_form_valid_pwd",
			Args:    []string{"--logical"},
			Env:     []string{"LC_ALL=C", "PWD=" + symlinkDir},
			WorkDir: physicalRealDir,
		},
		// === R1.2: -L falls back to physical when PWD is unset ===
		{
			Name:    "logical_flag_pwd_unset",
			Args:    []string{"-L"},
			Env:     []string{"LC_ALL=C", "PWD="},
			WorkDir: physicalRealDir,
		},
		// === R1.2: -L falls back when PWD contains .. component ===
		{
			Name:    "logical_flag_pwd_dotdot",
			Args:    []string{"-L"},
			Env:     []string{"LC_ALL=C", "PWD=" + physicalRealDir + "/.."},
			WorkDir: physicalRealDir,
		},
		// === R1.2: -L falls back when PWD is not absolute ===
		{
			Name:    "logical_flag_pwd_relative",
			Args:    []string{"-L"},
			Env:     []string{"LC_ALL=C", "PWD=relative/path"},
			WorkDir: physicalRealDir,
		},
		// === R1.2: -L falls back when PWD points to a different directory ===
		{
			Name:    "logical_flag_pwd_wrong_dir",
			Args:    []string{"-L"},
			Env:     []string{"LC_ALL=C", "PWD=/tmp"},
			WorkDir: physicalRealDir,
		},
		// === R1.4: when both -L and -P are given, last one wins ===
		{
			Name:    "last_flag_wins_LP",
			Args:    []string{"-L", "-P"},
			Env:     []string{"LC_ALL=C", "PWD=" + symlinkDir},
			WorkDir: physicalRealDir,
		},
		{
			Name:    "last_flag_wins_PL",
			Args:    []string{"-P", "-L"},
			Env:     []string{"LC_ALL=C", "PWD=" + symlinkDir},
			WorkDir: physicalRealDir,
		},
		// === R1.4: clustered flags, last in cluster wins ===
		{
			Name:    "clustered_LP",
			Args:    []string{"-LP"},
			Env:     []string{"LC_ALL=C", "PWD=" + symlinkDir},
			WorkDir: physicalRealDir,
		},
		{
			Name:    "clustered_PL",
			Args:    []string{"-PL"},
			Env:     []string{"LC_ALL=C", "PWD=" + symlinkDir},
			WorkDir: physicalRealDir,
		},
		// === R2.1: extra operand — gpwd ignores extra operands (exits 0). ===
		// === Implementation matches gpwd behavior.                      ===
		{
			Name:      "extra_operand_ignored",
			Args:      []string{"foo"},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   physicalRealDir,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		{
			Name:      "extra_operand_after_dashdash",
			Args:      []string{"--", "bar"},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   physicalRealDir,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		// === R2.2: unknown short flag produces error, exit 1 ===
		{
			Name:      "unknown_short_flag",
			Args:      []string{"-Z"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		// R2.2: unknown long flag.
		{
			Name:      "unknown_long_flag",
			Args:      []string{"--bogus-flag"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		// R2.2: unknown flag in a cluster with valid flags.
		{
			Name:      "unknown_flag_in_cluster",
			Args:      []string{"-LZ"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
