// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU du reference binary.
const refBinaryName = "gdu"

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// Create the standard fixture directory used by most tests.
	fixtureDir := createFixture(t)

	// du_default_recursive: default flags with -k on fixture directory.
	// R1.1, R1.2, R1.3.
	t.Run("du_default_recursive", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "default_k",
				Args:    []string{"-k", fixtureDir},
				WorkDir: t.TempDir(),
			},
		})
	})

	// du_summary_mode: -s prints only the total per argument. R2.2.
	t.Run("du_summary_mode", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "summary_k",
				Args:    []string{"-sk", fixtureDir},
				WorkDir: t.TempDir(),
			},
		})
	})

	// du_all_files: -a prints every file. R2.3.
	t.Run("du_all_files", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "all_k",
				Args:    []string{"-ak", fixtureDir},
				WorkDir: t.TempDir(),
			},
		})
	})

	// du_human_readable: -h displays human-readable sizes. R2.1.
	t.Run("du_human_readable", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "human",
				Args:    []string{"-h", fixtureDir},
				WorkDir: t.TempDir(),
			},
		})
	})

	// du_max_depth_1: -d 1 limits output depth. R2.4.
	t.Run("du_max_depth_1", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "depth_1",
				Args:    []string{"-k", "-d", "1", fixtureDir},
				WorkDir: t.TempDir(),
			},
		})
	})

	// du_grand_total: -c prints a total line. R2.7.
	t.Run("du_grand_total", func(t *testing.T) {
		sub1 := filepath.Join(fixtureDir, "subdir1")
		sub2 := filepath.Join(fixtureDir, "subdir2")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "grand_total",
				Args:    []string{"-ck", sub1, sub2},
				WorkDir: t.TempDir(),
			},
		})
	})

	// du_apparent_size: --apparent-size reports st_size. R2.8.
	t.Run("du_apparent_size", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "apparent_size",
				Args:    []string{"--apparent-size", "-k", fixtureDir},
				WorkDir: t.TempDir(),
			},
		})
	})

	// du_megabyte_units: -m reports in 1M blocks. R2.6.
	t.Run("du_megabyte_units", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "megabyte",
				Args:    []string{"-m", fixtureDir},
				WorkDir: t.TempDir(),
			},
		})
	})

	// du_hard_link_dedup: hard-linked file counted once. R3.1, R3.2, R3.3.
	t.Run("du_hard_link_dedup", func(t *testing.T) {
		hlDir := createHardlinkFixture(t)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "hardlink_dedup",
				Args:    []string{"-k", hlDir},
				WorkDir: t.TempDir(),
			},
		})
	})

	// du_missing_path: non-existent path prints error, exits 1. R4.2.
	// Normalize stderr: gdu prints "gdu:" while our binary prints "du:".
	t.Run("du_missing_path", func(t *testing.T) {
		stderrNorm := func(data []byte) []byte {
			return bytes.Replace(data, []byte("gdu:"), []byte("du:"), -1)
		}
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "missing_path",
				Args:      []string{"-k", filepath.Join(t.TempDir(), "nonexistent_dir")},
				WorkDir:   t.TempDir(),
				Normalize: []testutils.NormalizeFunc{stderrNorm},
			},
		})
	})
}

// createFixture builds a test directory tree with known file sizes.
// Structure:
//
//	du-fixture/
//	  subdir1/
//	    file_a.txt (1024 bytes)
//	    nested/
//	      file_b.txt (2048 bytes)
//	  subdir2/
//	    file_c.txt (512 bytes)
func createFixture(t *testing.T) string {
	t.Helper()

	base := filepath.Join(t.TempDir(), "du-fixture")
	dirs := []string{
		filepath.Join(base, "subdir1", "nested"),
		filepath.Join(base, "subdir2"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("create fixture dir: %v", err)
		}
	}

	files := map[string]int{
		filepath.Join(base, "subdir1", "file_a.txt"):          1024,
		filepath.Join(base, "subdir1", "nested", "file_b.txt"): 2048,
		filepath.Join(base, "subdir2", "file_c.txt"):          512,
	}
	for path, size := range files {
		data := make([]byte, size)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write fixture file: %v", err)
		}
	}

	return base
}

// createHardlinkFixture builds a directory with a hard-linked file.
// Structure:
//
//	du-hardlink-fixture/
//	  dir_a/
//	    shared.txt (4096 bytes)
//	  dir_b/
//	    shared.txt (hard link to dir_a/shared.txt)
func createHardlinkFixture(t *testing.T) string {
	t.Helper()

	base := filepath.Join(t.TempDir(), "du-hardlink-fixture")
	dirA := filepath.Join(base, "dir_a")
	dirB := filepath.Join(base, "dir_b")

	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatalf("create dir_a: %v", err)
	}
	if err := os.MkdirAll(dirB, 0o755); err != nil {
		t.Fatalf("create dir_b: %v", err)
	}

	srcFile := filepath.Join(dirA, "shared.txt")
	data := make([]byte, 4096)
	if err := os.WriteFile(srcFile, data, 0o644); err != nil {
		t.Fatalf("write shared.txt: %v", err)
	}

	linkFile := filepath.Join(dirB, "shared.txt")
	if err := os.Link(srcFile, linkFile); err != nil {
		t.Skipf("os.Link not supported: %v", err)
	}

	return base
}
