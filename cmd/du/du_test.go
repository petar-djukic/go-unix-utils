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
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
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
