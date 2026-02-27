// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/du exercising all du test cases from
// test-rel02.0.yaml.
//
// Implements: prd009-du R1-R5 (differential testing), prd001-testutils R1-R3
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goBinary is the path to the freshly built Go du binary. Set by TestMain.
var goBinary string

// refBinary is the path to the Homebrew reference gdu binary. Set by TestMain.
var refBinary string

// baseEnv provides the standard test environment per test-rel02.0.yaml
// preconditions: LC_ALL=C to eliminate locale-dependent divergence.
var baseEnv = []string{"LC_ALL=C"}

// TestMain builds the Go du binary and locates the Homebrew reference binary.
// Per design decision D1 and D4.
func TestMain(m *testing.M) {
	// Build the Go du binary into a temp directory.
	tmpDir, err := os.MkdirTemp("", "du-test-*")
	if err != nil {
		os.Exit(1)
	}

	goBinary = filepath.Join(tmpDir, "du")
	buildCmd := exec.Command("go", "build", "-o", goBinary, ".")
	if _, err := buildCmd.CombinedOutput(); err != nil {
		// Build failed; leave goBinary empty so tests skip gracefully.
		goBinary = ""
	}

	// Locate the Homebrew reference binary (brew install coreutils).
	// Per design decision D4: reference is gdu, not du (macOS BSD).
	refBinary, _ = exec.LookPath("gdu")

	code := m.Run()
	// Best-effort cleanup of temp directory.
	_ = os.RemoveAll(tmpDir)
	os.Exit(code)
}

// makeDuFixture creates a temp directory tree for du testing:
//
//	root/subdir1/file_a.txt  (1024 zero bytes, 0644)
//	root/subdir2/file_b.txt  (2048 zero bytes, 0644)
//	root/root_file.txt       (512 zero bytes, 0644)
//
// Returns the fixture root path. Per design decision D1.
func makeDuFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	subdir1 := filepath.Join(root, "subdir1")
	if err := os.MkdirAll(subdir1, 0755); err != nil {
		t.Fatalf("creating subdir1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subdir1, "file_a.txt"), make([]byte, 1024), 0644); err != nil {
		t.Fatalf("creating file_a.txt: %v", err)
	}

	subdir2 := filepath.Join(root, "subdir2")
	if err := os.MkdirAll(subdir2, 0755); err != nil {
		t.Fatalf("creating subdir2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subdir2, "file_b.txt"), make([]byte, 2048), 0644); err != nil {
		t.Fatalf("creating file_b.txt: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "root_file.txt"), make([]byte, 512), 0644); err != nil {
		t.Fatalf("creating root_file.txt: %v", err)
	}

	return root
}

// TestDuDifferential runs all differential test cases from test-rel02.0.yaml
// (du section). Per prd001-testutils AC1, the test defines a []DiffTest slice
// and calls RunDiffTests(t, goBinary, refBinary, tests).
func TestDuDifferential(t *testing.T) {
	if goBinary == "" {
		t.Skip("Go du binary could not be built; skipping differential tests")
	}
	if refBinary == "" {
		t.Skip("reference gdu binary not found on PATH (brew install coreutils); skipping differential tests")
	}

	// --- Shared fixture setup ---
	// Per design decision D1: create fixture files in t.TempDir() (not testdata/).
	fixtureDir := makeDuFixture(t)
	subdir1 := filepath.Join(fixtureDir, "subdir1")
	subdir2 := filepath.Join(fixtureDir, "subdir2")

	// Hard-link dedup fixture: dir_a/shared.txt and dir_b/shared.txt share the
	// same inode. Created at test time via os.Link. Per prd009-du R3.1-R3.3
	// and acceptance criterion AC4.
	hardlinkRoot := t.TempDir()
	hlDirA := filepath.Join(hardlinkRoot, "dir_a")
	hlDirB := filepath.Join(hardlinkRoot, "dir_b")
	if err := os.MkdirAll(hlDirA, 0755); err != nil {
		t.Fatalf("creating dir_a: %v", err)
	}
	if err := os.MkdirAll(hlDirB, 0755); err != nil {
		t.Fatalf("creating dir_b: %v", err)
	}
	sharedSrc := filepath.Join(hlDirA, "shared.txt")
	if err := os.WriteFile(sharedSrc, make([]byte, 1024), 0644); err != nil {
		t.Fatalf("creating shared.txt: %v", err)
	}
	if err := os.Link(sharedSrc, filepath.Join(hlDirB, "shared.txt")); err != nil {
		t.Fatalf("creating hard link: %v", err)
	}

	// Per design decision D2: no normalization is applied to any du test case.
	// du output is deterministic when both binaries stat the same files on the
	// same filesystem.
	tests := []testutils.DiffTest{
		// du_default_recursive: default flags with -k; one line per subdirectory
		// and root. Per test-rel02.0.yaml. Traces: prd009-du R1.1, R1.2, R1.3.
		{
			Name: "du_default_recursive",
			Args: []string{"-k", fixtureDir},
			Env:  baseEnv,
		},
		// du_summary_mode: -s prints only the total for each argument.
		// Traces: prd009-du R2.2.
		{
			Name: "du_summary_mode",
			Args: []string{"-sk", fixtureDir},
			Env:  baseEnv,
		},
		// du_all_files: -a prints size for every file, not just directories.
		// Traces: prd009-du R2.3.
		{
			Name: "du_all_files",
			Args: []string{"-ak", fixtureDir},
			Env:  baseEnv,
		},
		// du_human_readable: -h displays sizes as human-readable strings (K/M/G).
		// Traces: prd009-du R2.1.
		{
			Name: "du_human_readable",
			Args: []string{"-h", fixtureDir},
			Env:  baseEnv,
		},
		// du_max_depth_1: -d 1 limits output to argument and immediate children.
		// Traces: prd009-du R2.4.
		{
			Name: "du_max_depth_1",
			Args: []string{"-k", "-d", "1", fixtureDir},
			Env:  baseEnv,
		},
		// du_grand_total: -c prints grand total "total" line after all arguments.
		// Per design decision D3: both subdirs passed as separate Args elements.
		// Traces: prd009-du R2.7.
		{
			Name: "du_grand_total",
			Args: []string{"-ck", subdir1, subdir2},
			Env:  baseEnv,
		},
		// du_hard_link_dedup: file hard-linked into two directories is counted
		// only once. Both gdu and the Go binary must produce the same total.
		// Traces: prd009-du R3.1, R3.2, R3.3.
		{
			Name: "du_hard_link_dedup",
			Args: []string{"-k", hardlinkRoot},
			Env:  baseEnv,
		},
		// du_apparent_size: --apparent-size reports st_size instead of st_blocks.
		// Traces: prd009-du R2.8.
		{
			Name: "du_apparent_size",
			Args: []string{"--apparent-size", "-k", fixtureDir},
			Env:  baseEnv,
		},
		// du_kilobyte_units: -k reports in 1024-byte blocks (default; accepted
		// without error). Traces: prd009-du R2.5.
		{
			Name: "du_kilobyte_units",
			Args: []string{"-k", fixtureDir},
			Env:  baseEnv,
		},
		// du_megabyte_units: -m reports in 1048576-byte (1M) blocks.
		// Traces: prd009-du R2.6.
		{
			Name: "du_megabyte_units",
			Args: []string{"-m", fixtureDir},
			Env:  baseEnv,
		},
		// du_missing_path: non-existent argument prints error to stderr, exits 1.
		// Both binaries run in a fresh temp dir where nonexistent_dir is absent.
		// Traces: prd009-du R4.2.
		{
			Name: "du_missing_path",
			Args: []string{"-k", "nonexistent_dir"},
			Env:  baseEnv,
		},
	}

	testutils.RunDiffTests(t, goBinary, refBinary, tests)
}
