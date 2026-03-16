// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/readlink against greadlink (GNU coreutils).
// Implements prd050-readlink R1.1-R1.6, R2.1-R2.2, R3.1-R3.2, R4.1-R4.3 test coverage.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	// D2: graceful skip if greadlink is not installed.
	refBin, err := exec.LookPath("greadlink")
	if err != nil {
		t.Skipf("reference binary greadlink not in PATH: %v", err)
	}

	// Create a temp directory with symlinks for testing.
	tmpDir := t.TempDir()

	// Resolve tmpDir itself to handle macOS /private/var symlink.
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("resolving tmpDir: %v", err)
	}

	targetFile := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(targetFile, []byte("hello"), 0o644); err != nil {
		t.Fatalf("creating target file: %v", err)
	}

	// Simple symlink to a file.
	symlinkPath := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(targetFile, symlinkPath); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	// Relative symlink target.
	relSymlink := filepath.Join(tmpDir, "rellink.txt")
	if err := os.Symlink("target.txt", relSymlink); err != nil {
		t.Fatalf("creating relative symlink: %v", err)
	}

	// Symlink chain: chain -> link.txt -> target.txt.
	chainLink := filepath.Join(tmpDir, "chain.txt")
	if err := os.Symlink(symlinkPath, chainLink); err != nil {
		t.Fatalf("creating chain symlink: %v", err)
	}

	// Subdirectory for path tests.
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}

	// Symlink to directory.
	dirLink := filepath.Join(tmpDir, "dirlink")
	if err := os.Symlink(subDir, dirLink); err != nil {
		t.Fatalf("creating dir symlink: %v", err)
	}

	// D4: normalizer that strips stderr for error message format differences.
	normalizeStderr := func(b []byte) []byte {
		return nil
	}

	tests := []testutils.DiffTest{
		// R1.1: read symlink with absolute target.
		{
			Name:     "R1.1_symlink_absolute_target",
			Args:     []string{symlinkPath},
			ExitCode: 0,
		},
		// R1.1: read symlink with relative target.
		{
			Name:     "R1.1_symlink_relative_target",
			Args:     []string{relSymlink},
			ExitCode: 0,
		},
		// R1.2: non-symlink file exits 1.
		{
			Name:     "R1.2_non_symlink_file",
			Args:     []string{targetFile},
			ExitCode: 1,
		},
		// R1.2: directory (not a symlink) exits 1.
		{
			Name:     "R1.2_directory_not_symlink",
			Args:     []string{subDir},
			ExitCode: 1,
		},
		// R1.3: -f canonicalize follows symlink chain.
		{
			Name:     "R1.3_canonicalize_chain",
			Args:     []string{"-f", chainLink},
			ExitCode: 0,
		},
		// R1.3: -f with existing non-symlink path.
		{
			Name:     "R1.3_canonicalize_regular",
			Args:     []string{"-f", targetFile},
			ExitCode: 0,
		},
		// R1.3: -f with path where last component is missing.
		{
			Name:     "R1.3_canonicalize_missing_last",
			Args:     []string{"-f", filepath.Join(tmpDir, "nonexistent_file")},
			ExitCode: 0,
		},
		// R1.3: --canonicalize long form.
		{
			Name:     "R1.3_canonicalize_long",
			Args:     []string{"--canonicalize", chainLink},
			ExitCode: 0,
		},
		// R1.4: -e with existing path.
		{
			Name:     "R1.4_existing_path",
			Args:     []string{"-e", targetFile},
			ExitCode: 0,
		},
		// R1.4: -e with nonexistent path exits 1.
		{
			Name:      "R1.4_nonexistent_path",
			Args:      []string{"-e", filepath.Join(tmpDir, "no_such_file")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R1.4: --canonicalize-existing long form.
		{
			Name:     "R1.4_existing_long",
			Args:     []string{"--canonicalize-existing", targetFile},
			ExitCode: 0,
		},
		// R1.5: -m with completely nonexistent path.
		{
			Name:     "R1.5_missing_all",
			Args:     []string{"-m", filepath.Join(tmpDir, "x", "y", "z")},
			ExitCode: 0,
		},
		// R1.5: -m with existing path.
		{
			Name:     "R1.5_missing_existing",
			Args:     []string{"-m", targetFile},
			ExitCode: 0,
		},
		// R1.5: --canonicalize-missing long form.
		{
			Name:     "R1.5_missing_long",
			Args:     []string{"--canonicalize-missing", filepath.Join(tmpDir, "a", "b")},
			ExitCode: 0,
		},
		// R1.6: -n suppresses trailing newline for single operand.
		{
			Name:     "R1.6_no_newline",
			Args:     []string{"-n", symlinkPath},
			ExitCode: 0,
		},
		// R1.6: -n with -f mode.
		{
			Name:     "R1.6_no_newline_canonicalize",
			Args:     []string{"-nf", targetFile},
			ExitCode: 0,
		},
		// R2.1: multiple operands each printed on separate line.
		{
			Name:     "R2.1_multiple_symlinks",
			Args:     []string{symlinkPath, relSymlink},
			ExitCode: 0,
		},
		// R2.1: multiple operands with -f.
		{
			Name:     "R2.1_multiple_canonicalize",
			Args:     []string{"-f", symlinkPath, targetFile},
			ExitCode: 0,
		},
		// R2.2: -n is ignored with multiple operands (warning to stderr).
		{
			Name:      "R2.2_no_newline_ignored_multiple",
			Args:      []string{"-n", symlinkPath, relSymlink},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R3.1: no operands produces error, exit 1.
		{
			Name:      "R3.1_no_operands",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R3.2: unknown short flag.
		{
			Name:      "R3.2_unknown_short_flag",
			Args:      []string{"-x", symlinkPath},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R3.2: unknown long flag.
		{
			Name:      "R3.2_unknown_long_flag",
			Args:      []string{"--foobar", symlinkPath},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R1.3: -f through directory symlink.
		{
			Name:     "R1.3_canonicalize_dir_symlink",
			Args:     []string{"-f", dirLink},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
