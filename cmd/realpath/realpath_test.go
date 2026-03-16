// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/realpath against grealpath (GNU coreutils).
// Implements prd049-realpath R1.1-R1.4, R3.1, R3.3, R4.1-R4.3 test coverage.
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
	// D2: graceful skip if grealpath is not installed.
	refBin, err := exec.LookPath("grealpath")
	if err != nil {
		t.Skipf("reference binary grealpath not in PATH: %v", err)
	}

	// Create a temp directory with a symlink for symlink resolution tests.
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(targetFile, []byte("hello"), 0o644); err != nil {
		t.Fatalf("creating target file: %v", err)
	}
	symlinkPath := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(targetFile, symlinkPath); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	// Create a subdirectory for relative path tests.
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: resolve an existing absolute path.
		{
			Name:     "R1.1_absolute_path",
			Args:     []string{"/tmp"},
			ExitCode: 0,
		},
		// R1.1: resolve a relative path (dot).
		{
			Name:     "R1.1_relative_dot",
			Args:     []string{"."},
			WorkDir:  tmpDir,
			ExitCode: 0,
		},
		// R1.1: resolve a symlink to its target.
		{
			Name:     "R1.1_symlink_resolution",
			Args:     []string{symlinkPath},
			ExitCode: 0,
		},
		// R1.1: resolve path with .. component.
		{
			Name:     "R1.1_dotdot_component",
			Args:     []string{filepath.Join(subDir, "..")},
			ExitCode: 0,
		},
		// R1.2/R1.4: nonexistent path produces error, exit 1.
		{
			Name:      "R1.2_nonexistent_path",
			Args:      []string{"/nonexistent_path_xyz_abc_123"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R3.1: no operands produces usage error, exit 1.
		{
			Name:      "R3.1_no_operands",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R3.3: multiple paths, some failing — errors for failing, output for succeeding.
		{
			Name:      "R3.3_mixed_success_failure",
			Args:      []string{"/tmp", "/nonexistent_path_xyz_abc_123"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R1.1: multiple existing paths all resolve.
		{
			Name:     "R1.1_multiple_existing",
			Args:     []string{"/tmp", "/usr"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// normalizeStderr replaces all stderr output with empty bytes since the exact
// error message format may differ between implementations. Only exit codes
// and stdout are compared.
func normalizeStderr(b []byte) []byte {
	return nil
}
