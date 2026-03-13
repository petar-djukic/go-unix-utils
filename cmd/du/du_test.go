// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

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

	// Graceful skip per ARCHITECTURE.yaml shared protocol.
	refBin, err := exec.LookPath("gdu")
	if err != nil {
		t.Skipf("reference binary gdu not in PATH: %v", err)
	}

	// Build a fixture directory with a known structure so both binaries
	// operate on the same files on the same filesystem.
	fixture := t.TempDir()
	buildFixture(t, fixture)

	// Build a fixture with hard links for R3.1-R3.3 testing.
	hardlinkFixture := t.TempDir()
	buildHardlinkFixture(t, hardlinkFixture)

	tests := []testutils.DiffTest{
		{
			// Single file argument: both binaries should print one line.
			Name: "single_file_k",
			Args: []string{"-k", filepath.Join(fixture, "file.txt")},
		},
		{
			// Top-level directory: should print subdir line then root total.
			Name: "directory_k",
			Args: []string{"-k", fixture},
		},
		{
			// Subdirectory only: should print just the subdir total.
			Name: "nested_directory_k",
			Args: []string{"-k", filepath.Join(fixture, "subdir")},
		},
		{
			// R2.4: -d 1 limits output to argument and its immediate children only.
			Name: "max_depth_1",
			Args: []string{"-k", "-d", "1", fixture},
		},
		{
			// R2.4: --max-depth=0 is equivalent to -s (only the argument total).
			Name: "max_depth_0",
			Args: []string{"-k", "--max-depth=0", fixture},
		},
		{
			// R2.4: -d 0 is equivalent to -s via short flag.
			Name: "max_depth_0_short",
			Args: []string{"-k", "-d", "0", fixture},
		},
		{
			// R2.6: -m reports sizes in 1M blocks.
			Name: "mega_blocks",
			Args: []string{"-m", fixture},
		},
		{
			// R2.7: -c prints grand total after a single directory argument.
			Name: "grand_total_single",
			Args: []string{"-k", "-c", fixture},
		},
		{
			// R2.7: -c with two arguments prints a total line at the end.
			Name: "grand_total_two_args",
			Args: []string{"-k", "-c", fixture, filepath.Join(fixture, "subdir")},
		},
		{
			// R2.7 + R2.1: -c with -h formats grand total as human-readable.
			Name: "grand_total_human",
			Args: []string{"-h", "-c", fixture},
		},
		{
			// R2.8: --apparent-size reports st_size instead of st_blocks.
			Name: "apparent_size",
			Args: []string{"--apparent-size", "-k", fixture},
		},
		{
			// R2.8: --apparent-size with -a shows apparent sizes for all files.
			Name: "apparent_size_all_files",
			Args: []string{"--apparent-size", "-a", "-k", fixture},
		},
		{
			// R2.8: --apparent-size combined with -h for human-readable output.
			Name: "apparent_size_human",
			Args: []string{"--apparent-size", "-h", fixture},
		},
		{
			// R2.8: --apparent-size with -s shows apparent size total only.
			Name: "apparent_size_summary",
			Args: []string{"--apparent-size", "-s", "-k", fixture},
		},
		{
			// R3.1, R3.2: hard-linked file counted only once; summary avoids
			// traversal-order dependency between Go (alphabetical) and gdu (readdir).
			Name: "hardlink_dedup_summary",
			Args: []string{"-s", "-k", hardlinkFixture},
		},
		{
			// R3.3: cross-argument dedup — same file via two hard links passed as
			// separate arguments. The second argument contributes 0 since the inode
			// was already counted by the first. Command-line order is deterministic.
			Name: "hardlink_dedup_cross_arg_file",
			Args: []string{"-k", "-c",
				filepath.Join(hardlinkFixture, "dir1", "original.txt"),
				filepath.Join(hardlinkFixture, "dir2", "linked.txt")},
		},
		{
			// R3.3: cross-argument dedup with --apparent-size.
			Name: "hardlink_dedup_cross_arg_apparent",
			Args: []string{"--apparent-size", "-k", "-c",
				filepath.Join(hardlinkFixture, "dir1", "original.txt"),
				filepath.Join(hardlinkFixture, "dir2", "linked.txt")},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// buildHardlinkFixture creates a directory structure with hard links for R3.1-R3.3 testing.
// Layout:
//
//	dir/dir1/original.txt (file with content)
//	dir/dir2/linked.txt   (hard link to dir1/original.txt)
func buildHardlinkFixture(t *testing.T, dir string) {
	t.Helper()

	dir1 := filepath.Join(dir, "dir1")
	if err := os.Mkdir(dir1, 0o755); err != nil {
		t.Fatalf("buildHardlinkFixture: %v", err)
	}
	original := filepath.Join(dir1, "original.txt")
	if err := os.WriteFile(original, []byte("hardlink test content\n"), 0o644); err != nil {
		t.Fatalf("buildHardlinkFixture: %v", err)
	}

	dir2 := filepath.Join(dir, "dir2")
	if err := os.Mkdir(dir2, 0o755); err != nil {
		t.Fatalf("buildHardlinkFixture: %v", err)
	}
	linked := filepath.Join(dir2, "linked.txt")
	if err := os.Link(original, linked); err != nil {
		t.Fatalf("buildHardlinkFixture: %v", err)
	}
}

// buildFixture creates a reproducible directory structure for differential testing.
func buildFixture(t *testing.T, dir string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("buildFixture: %v", err)
	}

	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatalf("buildFixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "inner.txt"), []byte("inner\n"), 0o644); err != nil {
		t.Fatalf("buildFixture: %v", err)
	}
}
