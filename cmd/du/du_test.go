// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	// Normalizer for stderr: gdu prints "gdu:" while our binary prints "du:".
	stderrNorm := func(data []byte) []byte {
		return bytes.Replace(data, []byte("gdu:"), []byte("du:"), -1)
	}

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
	t.Run("du_missing_path", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "missing_path",
				Args:      []string{"-k", filepath.Join(t.TempDir(), "nonexistent_dir")},
				WorkDir:   t.TempDir(),
				Normalize: []testutils.NormalizeFunc{stderrNorm},
			},
		})
	})

	// du_empty_directory: empty dir reports only the directory itself. R1.1.
	t.Run("du_empty_directory", func(t *testing.T) {
		emptyDir := filepath.Join(t.TempDir(), "empty")
		if err := os.MkdirAll(emptyDir, 0o755); err != nil {
			t.Fatalf("create empty dir: %v", err)
		}
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "empty_dir",
				Args:    []string{"-k", emptyDir},
				WorkDir: t.TempDir(),
			},
		})
	})

	// du_single_file: du on a single file reports its size. R1.1, R2.3.
	t.Run("du_single_file", func(t *testing.T) {
		singleFile := filepath.Join(t.TempDir(), "single.txt")
		if err := os.WriteFile(singleFile, make([]byte, 4096), 0o644); err != nil {
			t.Fatalf("write single file: %v", err)
		}
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "single_file",
				Args:    []string{"-k", singleFile},
				WorkDir: t.TempDir(),
			},
		})
	})

	// du_permission_denied: unreadable directory prints error, exits 1. R4.2.
	t.Run("du_permission_denied", func(t *testing.T) {
		if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
			t.Skip("permission test requires unix")
		}
		if os.Getuid() == 0 {
			t.Skip("permission test not meaningful as root")
		}
		base := filepath.Join(t.TempDir(), "perm-fixture")
		restricted := filepath.Join(base, "noaccess")
		if err := os.MkdirAll(restricted, 0o755); err != nil {
			t.Fatalf("create restricted dir: %v", err)
		}
		// Write a file inside before restricting access.
		if err := os.WriteFile(filepath.Join(restricted, "secret.txt"), make([]byte, 1024), 0o644); err != nil {
			t.Fatalf("write secret file: %v", err)
		}
		if err := os.Chmod(restricted, 0o000); err != nil {
			t.Fatalf("chmod restricted dir: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Chmod(restricted, 0o755) // best-effort restore for cleanup
		})
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "permission_denied",
				Args:      []string{"-k", base},
				WorkDir:   t.TempDir(),
				Normalize: []testutils.NormalizeFunc{stderrNorm},
			},
		})
	})

	// du_mixed_valid_invalid: mix of valid dir and nonexistent path. R4.2.
	t.Run("du_mixed_valid_invalid", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "mixed_args",
				Args:      []string{"-k", fixtureDir, filepath.Join(t.TempDir(), "no_such_dir")},
				WorkDir:   t.TempDir(),
				Normalize: []testutils.NormalizeFunc{stderrNorm},
			},
		})
	})

	// du_max_depth_0: -d 0 equivalent to -s. R2.4.
	t.Run("du_max_depth_0", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "depth_0",
				Args:    []string{"-k", "-d", "0", fixtureDir},
				WorkDir: t.TempDir(),
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
		filepath.Join(base, "subdir1", "file_a.txt"):           1024,
		filepath.Join(base, "subdir1", "nested", "file_b.txt"): 2048,
		filepath.Join(base, "subdir2", "file_c.txt"):           512,
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
