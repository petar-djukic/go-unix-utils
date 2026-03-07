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

// refBinaryName is the name of the Homebrew GNU du reference binary.
const refBinaryName = "gdu"

// stderrNormalizer replaces "gdu:" with "du:" so stderr comparison ignores
// the binary name prefix difference.
var stderrNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gdu:"), []byte("du:"))
}

// setupFixture creates a test fixture directory with known structure:
//
//	du-fixture/
//	  subdir1/
//	    file1.txt (1024 bytes)
//	    file2.txt (2048 bytes)
//	  subdir2/
//	    file3.txt (4096 bytes)
//	  topfile.txt (512 bytes)
func setupFixture(t *testing.T) string {
	t.Helper()
	base := filepath.Join(t.TempDir(), "du-fixture")

	sub1 := filepath.Join(base, "subdir1")
	sub2 := filepath.Join(base, "subdir2")

	if err := os.MkdirAll(sub1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sub2, 0o755); err != nil {
		t.Fatal(err)
	}

	writeBytes(t, filepath.Join(sub1, "file1.txt"), 1024)
	writeBytes(t, filepath.Join(sub1, "file2.txt"), 2048)
	writeBytes(t, filepath.Join(sub2, "file3.txt"), 4096)
	writeBytes(t, filepath.Join(base, "topfile.txt"), 512)

	return base
}

// setupHardlinkFixture creates a fixture with a hard-linked file:
//
//	du-hardlink-fixture/
//	  dir_a/
//	    shared.txt (4096 bytes)
//	  dir_b/
//	    shared.txt (hard link to dir_a/shared.txt)
func setupHardlinkFixture(t *testing.T) string {
	t.Helper()
	base := filepath.Join(t.TempDir(), "du-hardlink-fixture")

	dirA := filepath.Join(base, "dir_a")
	dirB := filepath.Join(base, "dir_b")

	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dirB, 0o755); err != nil {
		t.Fatal(err)
	}

	writeBytes(t, filepath.Join(dirA, "shared.txt"), 4096)

	if err := os.Link(filepath.Join(dirA, "shared.txt"), filepath.Join(dirB, "shared.txt")); err != nil {
		t.Fatalf("failed to create hard link: %v", err)
	}

	return base
}

// setupSymlinkFixture creates a fixture with a symbolic link to verify
// that du does not follow symlinks during traversal.
func setupSymlinkFixture(t *testing.T) string {
	t.Helper()
	base := filepath.Join(t.TempDir(), "du-symlink-fixture")
	sub := filepath.Join(base, "realdir")

	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	writeBytes(t, filepath.Join(sub, "file.txt"), 2048)

	if err := os.Symlink(sub, filepath.Join(base, "linkdir")); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	return base
}

// writeBytes writes n zero bytes to the given path.
func writeBytes(t *testing.T, path string, n int) {
	t.Helper()
	data := make([]byte, n)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := setupFixture(t)
	hlFixture := setupHardlinkFixture(t)
	slFixture := setupSymlinkFixture(t)

	tests := []testutils.DiffTest{
		{
			Name: "du_default_recursive",
			Args: []string{"-k", fixture},
		},
		{
			Name: "du_summary_mode",
			Args: []string{"-sk", fixture},
		},
		{
			Name: "du_all_files",
			Args: []string{"-ak", fixture},
		},
		{
			Name: "du_human_readable",
			Args: []string{"-h", fixture},
		},
		{
			Name: "du_max_depth_1",
			Args: []string{"-k", "-d", "1", fixture},
		},
		{
			Name: "du_grand_total",
			Args: []string{"-ck", filepath.Join(fixture, "subdir1"), filepath.Join(fixture, "subdir2")},
		},
		{
			Name: "du_hard_link_dedup",
			Args: []string{"-k", hlFixture},
		},
		{
			Name: "du_apparent_size",
			Args: []string{"--apparent-size", "-k", fixture},
		},
		{
			Name: "du_kilobyte_units",
			Args: []string{"-k", fixture},
		},
		{
			Name: "du_megabyte_units",
			Args: []string{"-m", fixture},
		},
		{
			Name:      "du_missing_path",
			Args:      []string{"-k", "nonexistent_dir_xyzzy"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		{
			Name: "du_multiple_args",
			Args: []string{"-k", filepath.Join(fixture, "subdir1"), filepath.Join(fixture, "subdir2")},
		},
		{
			Name: "du_symlink_not_followed",
			Args: []string{"-ak", slFixture},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
