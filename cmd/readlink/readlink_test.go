// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd050-readlink R4.1–R4.3 (differential tests for R1.1–R1.6, R2.1–R2.2)
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for readlink.
const refBinaryName = "greadlink"

// TestDiff tests R1.1–R1.6, R2.1–R2.2: default readlink behavior with symlinks,
// non-symlinks, canonicalization modes, -n flag, and multiple operands.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// Create a temporary directory with symlinks for testing.
	tmpDir := t.TempDir()

	// Create a regular file.
	regularFile := filepath.Join(tmpDir, "regular.txt")
	if err := os.WriteFile(regularFile, []byte("hello"), 0o644); err != nil {
		t.Fatalf("creating regular file: %v", err)
	}

	// Create a symlink pointing to the regular file.
	symlinkFile := filepath.Join(tmpDir, "link_to_regular")
	if err := os.Symlink(regularFile, symlinkFile); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	// Create a symlink pointing to a relative target.
	relSymlink := filepath.Join(tmpDir, "link_relative")
	if err := os.Symlink("regular.txt", relSymlink); err != nil {
		t.Fatalf("creating relative symlink: %v", err)
	}

	// Create a dangling symlink (target does not exist).
	danglingSymlink := filepath.Join(tmpDir, "link_dangling")
	if err := os.Symlink("/nonexistent_target_xyz_12345", danglingSymlink); err != nil {
		t.Fatalf("creating dangling symlink: %v", err)
	}

	// Create a subdirectory.
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}

	// Create a symlink to the subdirectory.
	symlinkDir := filepath.Join(tmpDir, "link_to_dir")
	if err := os.Symlink(subDir, symlinkDir); err != nil {
		t.Fatalf("creating dir symlink: %v", err)
	}

	// Create a chained symlink: link_chain -> link_to_regular -> regular.txt.
	chainedSymlink := filepath.Join(tmpDir, "link_chain")
	if err := os.Symlink(symlinkFile, chainedSymlink); err != nil {
		t.Fatalf("creating chained symlink: %v", err)
	}

	// Path that does not exist at all (for -m testing).
	missingPath := filepath.Join(tmpDir, "no_such_dir", "no_such_file")

	// Path where parent exists but file does not (for -f testing).
	partialPath := filepath.Join(tmpDir, "nonexistent_file")

	// Normalize stderr since error messages may differ in formatting.
	clearStderr := func(b []byte) []byte { return nil }

	tests := []testutils.DiffTest{
		// === R1.1: symlink with absolute target prints the target ===
		{
			Name: "symlink_absolute_target",
			Args: []string{symlinkFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: symlink with relative target prints the relative target.
		{
			Name: "symlink_relative_target",
			Args: []string{relSymlink},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: dangling symlink still prints the target (target need not exist).
		{
			Name: "dangling_symlink",
			Args: []string{danglingSymlink},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: symlink to directory prints the target.
		{
			Name: "symlink_to_dir",
			Args: []string{symlinkDir},
			Env:  []string{"LC_ALL=C"},
		},
		// === R1.2: regular file is not a symlink — exit 1 ===
		{
			Name:      "regular_file_not_symlink",
			Args:      []string{regularFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		// R1.2: directory is not a symlink — exit 1.
		{
			Name:      "directory_not_symlink",
			Args:      []string{subDir},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		// R1.2: nonexistent path — exit 1.
		{
			Name:      "nonexistent_path",
			Args:      []string{filepath.Join(tmpDir, "does_not_exist")},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		// === R1.3: -f canonicalize — resolve full path, last component may not exist ===
		{
			Name: "canon_f_symlink",
			Args: []string{"-f", symlinkFile},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "canon_f_chained_symlink",
			Args: []string{"-f", chainedSymlink},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "canon_f_partial_path",
			Args: []string{"-f", partialPath},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name:      "canon_f_missing_parent",
			Args:      []string{"-f", missingPath},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		// === R1.4: -e canonicalize-existing — every component must exist ===
		{
			Name: "canon_e_existing_file",
			Args: []string{"-e", regularFile},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "canon_e_symlink",
			Args: []string{"-e", symlinkFile},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name:      "canon_e_missing_file",
			Args:      []string{"-e", partialPath},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		{
			Name:      "canon_e_dangling_symlink",
			Args:      []string{"-e", danglingSymlink},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		// === R1.5: -m canonicalize-missing — no component need exist ===
		{
			Name: "canon_m_existing_symlink",
			Args: []string{"-m", symlinkFile},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "canon_m_missing_path",
			Args: []string{"-m", missingPath},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "canon_m_partial_path",
			Args: []string{"-m", partialPath},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "canon_m_long_form",
			Args: []string{"--canonicalize-missing", symlinkFile},
			Env:  []string{"LC_ALL=C"},
		},
		// === R1.6: -n suppress trailing newline for single operand ===
		{
			Name: "no_newline_single_symlink",
			Args: []string{"-n", symlinkFile},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "no_newline_long_form",
			Args: []string{"--no-newline", symlinkFile},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "no_newline_with_canon_f",
			Args: []string{"-n", "-f", symlinkFile},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "no_newline_with_canon_m",
			Args: []string{"-n", "-m", missingPath},
			Env:  []string{"LC_ALL=C"},
		},
		// === R2.1: multiple operands — each result on a separate line ===
		{
			Name: "multiple_symlinks",
			Args: []string{symlinkFile, relSymlink, danglingSymlink},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4/R2.1: mixed operands — first succeeds, second fails, third succeeds — exit 1.
		{
			Name:      "mixed_success_failure",
			Args:      []string{symlinkFile, regularFile, relSymlink},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		// R2.1: first fails, second succeeds — exit 1.
		{
			Name:      "first_fails_second_ok",
			Args:      []string{regularFile, symlinkFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		// R2.1: all fail — exit 1.
		{
			Name:      "all_fail",
			Args:      []string{regularFile, subDir},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		// R2.1: multiple operands with -f.
		{
			Name: "multiple_operands_canon_f",
			Args: []string{"-f", symlinkFile, chainedSymlink},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: multiple operands with -m.
		{
			Name: "multiple_operands_canon_m",
			Args: []string{"-m", symlinkFile, missingPath},
			Env:  []string{"LC_ALL=C"},
		},
		// === R2.2: -n is ignored with multiple operands ===
		{
			Name:      "no_newline_ignored_multiple",
			Args:      []string{"-n", symlinkFile, relSymlink},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		{
			Name:      "no_newline_ignored_multiple_canon_f",
			Args:      []string{"-n", "-f", symlinkFile, chainedSymlink},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
