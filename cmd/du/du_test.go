// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinary is the Homebrew GNU du reference binary name.
const refBinary = "gdu"

// setupFixture creates a test directory structure with known files.
// fixture/
// ├── subdir1/
// │   ├── file1.txt
// │   └── file2.txt
// └── subdir2/
//     └── file3.txt
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

	files := map[string]string{
		filepath.Join(baseDir, "fixture", "subdir1", "file1.txt"): strings.Repeat("a", 4096),
		filepath.Join(baseDir, "fixture", "subdir1", "file2.txt"): strings.Repeat("b", 8192),
		filepath.Join(baseDir, "fixture", "subdir2", "file3.txt"): strings.Repeat("c", 2048),
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
}

// setupHardlinkFixture creates a directory with a hard-linked file for dedup testing.
// hardlink-fixture/
// ├── dir_a/
// │   └── shared.txt
// └── dir_b/
//     └── shared.txt  (hard link to dir_a/shared.txt)
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
	if err := os.WriteFile(srcFile, []byte(strings.Repeat("x", 4096)), 0o644); err != nil {
		t.Fatalf("writing %s: %v", srcFile, err)
	}

	linkFile := filepath.Join(dirB, "shared.txt")
	if err := os.Link(srcFile, linkFile); err != nil {
		t.Fatalf("creating hard link %s -> %s: %v", linkFile, srcFile, err)
	}
}

// stderrNormalizer replaces error messages with a canonical form so stderr
// comparison passes despite formatting differences between GNU du and Go du.
// Both include the path; we normalize the surrounding text.
func stderrNormalizer(b []byte) []byte {
	s := string(b)
	// Normalize "du: cannot access 'path': ..." vs "du: cannot access 'path': ..."
	// Both should produce compatible messages, but case may differ in the OS error.
	s = strings.ToLower(s)
	return []byte(s)
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinary)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinary, err)
	}

	// Create shared fixture directory.
	baseDir := t.TempDir()
	setupFixture(t, baseDir)
	setupHardlinkFixture(t, baseDir)

	tests := []testutils.DiffTest{
		{
			Name:    "du_default_recursive",
			Args:    []string{"-k", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "du_summary_mode",
			Args:    []string{"-sk", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "du_all_files",
			Args:    []string{"-ak", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "du_human_readable",
			Args:    []string{"-h", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "du_max_depth_1",
			Args:    []string{"-k", "-d", "1", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "du_grand_total",
			Args:    []string{"-ck", "fixture/subdir1", "fixture/subdir2"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "du_hard_link_dedup",
			Args:    []string{"-k", "hardlink-fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "du_apparent_size",
			Args:    []string{"--apparent-size", "-k", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "du_megabyte_units",
			Args:    []string{"-m", "fixture"},
			WorkDir: baseDir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:      "du_missing_path",
			Args:      []string{"-k", "nonexistent_dir"},
			WorkDir:   baseDir,
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
