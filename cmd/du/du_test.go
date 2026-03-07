// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/du covering prd009-du R1-R4.
// Verifies Go du against Homebrew GNU gdu (coreutils).
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for du.
const refBinaryName = "gdu"

// binaryNameNormalizer replaces the "gdu:" prefix in stderr with "du:" so
// error messages from both binaries can be compared byte-for-byte.
var binaryNameNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return []byte(strings.ReplaceAll(string(b), "gdu:", "du:"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// Create shared fixture directory tree.
	baseDir := t.TempDir()
	setupFixture(t, baseDir)
	setupHardlinkFixture(t, baseDir)
	setupSymlinkFixture(t, baseDir)

	tests := []testutils.DiffTest{
		// R1: Directory traversal
		{
			Name:    "default_recursive",
			Args:    []string{"-k", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "nested_subdir",
			Args:    []string{"-k", "fixture/subdir1"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "multiple_dir_args",
			Args:    []string{"-k", "fixture/subdir1", "fixture/subdir2"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "single_file_arg",
			Args:    []string{"-k", "fixture/subdir1/file1.txt"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "symlink_not_followed",
			Args:    []string{"-k", "symlink-fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},

		// R2: Block counting
		{
			Name:    "kilobyte_blocks",
			Args:    []string{"-k", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "megabyte_blocks",
			Args:    []string{"-m", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "apparent_size",
			Args:    []string{"--apparent-size", "-k", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "apparent_size_megabytes",
			Args:    []string{"--apparent-size", "-m", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},

		// R3: Human-readable output
		{
			Name:    "human_readable",
			Args:    []string{"-h", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "human_readable_summary",
			Args:    []string{"-h", "-s", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "human_readable_all_files",
			Args:    []string{"-h", "-a", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},

		// R4: Summarize mode, grand total, depth limit
		{
			Name:    "summary_mode",
			Args:    []string{"-sk", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "grand_total",
			Args:    []string{"-ck", "fixture/subdir1", "fixture/subdir2"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "summary_grand_total",
			Args:    []string{"-s", "-c", "-k", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "max_depth_1",
			Args:    []string{"-k", "-d", "1", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},

		// R1/R3: Hard-link deduplication
		{
			Name:    "hardlink_dedup",
			Args:    []string{"-k", "hardlink-fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},

		// R2: All-files mode
		{
			Name:    "all_files",
			Args:    []string{"-ak", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},

		// R4: Exit codes — error on missing path
		{
			Name:      "missing_path",
			Args:      []string{"-k", "nonexistent_dir"},
			WorkDir:   baseDir,
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{binaryNameNormalizer},
		},
		{
			Name:      "missing_and_valid_path",
			Args:      []string{"-k", "nonexistent_dir", "fixture/subdir1"},
			WorkDir:   baseDir,
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{binaryNameNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// setupFixture creates a directory tree for du tests:
//
//	fixture/
//	  subdir1/
//	    file1.txt (4096 bytes)
//	    file2.txt (8192 bytes)
//	  subdir2/
//	    file3.txt (2048 bytes)
func setupFixture(t *testing.T, baseDir string) {
	t.Helper()
	dirs := []string{
		filepath.Join(baseDir, "fixture", "subdir1"),
		filepath.Join(baseDir, "fixture", "subdir2"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("creating directory %s: %v", d, err)
		}
	}
	writeFixture(t, filepath.Join(baseDir, "fixture", "subdir1", "file1.txt"), 4096)
	writeFixture(t, filepath.Join(baseDir, "fixture", "subdir1", "file2.txt"), 8192)
	writeFixture(t, filepath.Join(baseDir, "fixture", "subdir2", "file3.txt"), 2048)
}

// setupHardlinkFixture creates a fixture with a hard-linked file:
//
//	hardlink-fixture/
//	  dir_a/
//	    shared.txt (4096 bytes)
//	  dir_b/
//	    shared.txt (hard link to dir_a/shared.txt)
func setupHardlinkFixture(t *testing.T, baseDir string) {
	t.Helper()
	dirA := filepath.Join(baseDir, "hardlink-fixture", "dir_a")
	dirB := filepath.Join(baseDir, "hardlink-fixture", "dir_b")
	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatalf("creating %s: %v", dirA, err)
	}
	if err := os.MkdirAll(dirB, 0o755); err != nil {
		t.Fatalf("creating %s: %v", dirB, err)
	}
	srcFile := filepath.Join(dirA, "shared.txt")
	writeFixture(t, srcFile, 4096)
	linkFile := filepath.Join(dirB, "shared.txt")
	if err := os.Link(srcFile, linkFile); err != nil {
		t.Fatalf("creating hard link %s -> %s: %v", linkFile, srcFile, err)
	}
}

// setupSymlinkFixture creates a fixture with a symlink to test R1.4:
//
//	symlink-fixture/
//	  realdir/
//	    file.txt (512 bytes)
//	  link -> realdir (symlink, must not be followed)
func setupSymlinkFixture(t *testing.T, baseDir string) {
	t.Helper()
	realDir := filepath.Join(baseDir, "symlink-fixture", "realdir")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatalf("creating %s: %v", realDir, err)
	}
	writeFixture(t, filepath.Join(realDir, "file.txt"), 512)
	linkPath := filepath.Join(baseDir, "symlink-fixture", "link")
	if err := os.Symlink("realdir", linkPath); err != nil {
		t.Fatalf("creating symlink %s: %v", linkPath, err)
	}
}

// writeFixture creates a file of the given size filled with 'A' bytes.
func writeFixture(t *testing.T, path string, size int) {
	t.Helper()
	data := make([]byte, size)
	for i := range data {
		data[i] = 'A'
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
