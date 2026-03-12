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
