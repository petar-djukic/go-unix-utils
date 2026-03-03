// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/du differential tests verify output parity between the Go du binary and
// the GNU reference binary gdu (Homebrew coreutils). All tests run with
// LC_ALL=C to eliminate locale-dependent divergence. Block-count tests use -k
// to pin output to 1024-byte blocks.
//
// Implements: prd009-du R1-R4
// Architecture: docs/ARCHITECTURE.yaml (cmd/ component, DD2, DD4, DD6)
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinaryName = "gdu"

var (
	goBin  string
	refBin string
)

func TestMain(m *testing.M) {
	ref, err := exec.LookPath(refBinaryName)
	if err == nil {
		refBin = ref
	}

	tmpDir, err := os.MkdirTemp("", "du-test-*")
	if err != nil {
		os.Stderr.WriteString("failed to create temp dir: " + err.Error() + "\n")
		os.Exit(1)
	}

	goBin = filepath.Join(tmpDir, "du")
	build := exec.Command("go", "build", "-o", goBin, ".")
	if out, buildErr := build.CombinedOutput(); buildErr != nil {
		os.RemoveAll(tmpDir) // best-effort cleanup
		os.Stderr.WriteString("go build failed: " + string(out) + "\n")
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmpDir) // best-effort cleanup
	os.Exit(code)
}

// skipIfMissing skips the current test when gdu is not available on PATH.
// (AC6: tests skip gracefully)
func skipIfMissing(t *testing.T) {
	t.Helper()
	if refBin == "" {
		t.Skip(refBinaryName + " not found in PATH")
	}
}

// createFixture builds a directory tree for du differential tests.
//
// Layout:
//
//	fixture/
//	  subdir1/
//	    file1.txt (4096 bytes)
//	    nested/
//	      file2.txt (2048 bytes)
//	  subdir2/
//	    file3.txt (8192 bytes)
func createFixture(t *testing.T, base string) {
	t.Helper()
	dirs := []string{
		filepath.Join(base, "fixture", "subdir1"),
		filepath.Join(base, "fixture", "subdir1", "nested"),
		filepath.Join(base, "fixture", "subdir2"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	files := []struct {
		rel  string
		size int
	}{
		{"fixture/subdir1/file1.txt", 4096},
		{"fixture/subdir1/nested/file2.txt", 2048},
		{"fixture/subdir2/file3.txt", 8192},
	}
	for _, f := range files {
		p := filepath.Join(base, f.rel)
		if err := os.WriteFile(p, bytes.Repeat([]byte("X"), f.size), 0644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
}

// createHardLinkFixture builds a directory tree with a hard-linked file to
// test deduplication. dir_a/shared.txt and dir_b/shared.txt are the same inode.
//
// Layout:
//
//	hlfix/
//	  dir_a/
//	    shared.txt (4096 bytes)
//	  dir_b/
//	    shared.txt (hard link to dir_a/shared.txt)
func createHardLinkFixture(t *testing.T, base string) {
	t.Helper()
	dirA := filepath.Join(base, "hlfix", "dir_a")
	dirB := filepath.Join(base, "hlfix", "dir_b")
	if err := os.MkdirAll(dirA, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dirA, err)
	}
	if err := os.MkdirAll(dirB, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dirB, err)
	}

	src := filepath.Join(dirA, "shared.txt")
	if err := os.WriteFile(src, bytes.Repeat([]byte("D"), 4096), 0644); err != nil {
		t.Fatalf("write %s: %v", src, err)
	}
	if err := os.Link(src, filepath.Join(dirB, "shared.txt")); err != nil {
		t.Fatalf("link: %v", err)
	}
}

// progNameNormalizer replaces "gdu:" with "du:" in output so error messages
// from the GNU reference binary (installed as gdu) match the Go binary's
// "du:" prefix.
func progNameNormalizer(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gdu:"), []byte("du:"))
}

// errPresenceNormalizer replaces any non-empty output with a fixed marker.
// Used for test cases where stderr format differs between implementations but
// both must produce non-empty error output.
func errPresenceNormalizer(b []byte) []byte {
	if len(b) > 0 {
		return []byte("OUTPUT\n")
	}
	return b
}

// TestDuDefaultRecursive tests basic disk usage reporting: directory traversal
// with bottom-up accumulation, file operand handling, and multiple arguments.
// (prd009-du R1.1, R1.2, R1.3, R1.4, R1.5)
func TestDuDefaultRecursive(t *testing.T) {
	skipIfMissing(t)

	base := t.TempDir()
	createFixture(t, base)

	tests := []testutils.DiffTest{
		{
			Name:     "default_recursive",
			Args:     []string{"-k", "fixture"},
			WorkDir:  base,
			ExitCode: 0,
		},
		{
			Name:     "file_operand",
			Args:     []string{"-k", "fixture/subdir1/file1.txt"},
			WorkDir:  base,
			ExitCode: 0,
		},
		{
			Name:     "multiple_arguments",
			Args:     []string{"-k", "fixture/subdir1", "fixture/subdir2"},
			WorkDir:  base,
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDuFlags tests all flag behaviors per prd009-du R2: human-readable (-h),
// all files (-a), max depth (-d N, --max-depth=N), kilo blocks (-k), mega
// blocks (-m), grand total (-c), apparent size (--apparent-size), summarize
// (-s), and combined flags.
// (prd009-du R2.1, R2.3, R2.4, R2.5, R2.6, R2.7, R2.8)
func TestDuFlags(t *testing.T) {
	skipIfMissing(t)

	base := t.TempDir()
	createFixture(t, base)

	tests := []testutils.DiffTest{
		{
			Name:     "human_readable",
			Args:     []string{"-h", "fixture"},
			WorkDir:  base,
			ExitCode: 0,
		},
		{
			Name:     "all_files",
			Args:     []string{"-ak", "fixture"},
			WorkDir:  base,
			ExitCode: 0,
		},
		{
			Name:     "max_depth_short",
			Args:     []string{"-k", "-d", "1", "fixture"},
			WorkDir:  base,
			ExitCode: 0,
		},
		{
			Name:     "max_depth_long",
			Args:     []string{"-k", "--max-depth=1", "fixture"},
			WorkDir:  base,
			ExitCode: 0,
		},
		{
			Name:     "kilo_blocks",
			Args:     []string{"-k", "fixture"},
			WorkDir:  base,
			ExitCode: 0,
		},
		{
			Name:     "mega_blocks",
			Args:     []string{"-m", "fixture"},
			WorkDir:  base,
			ExitCode: 0,
		},
		{
			Name:     "grand_total",
			Args:     []string{"-ck", "fixture/subdir1", "fixture/subdir2"},
			WorkDir:  base,
			ExitCode: 0,
		},
		{
			Name:     "apparent_size",
			Args:     []string{"--apparent-size", "-k", "fixture"},
			WorkDir:  base,
			ExitCode: 0,
		},
		{
			Name:     "summary_mode",
			Args:     []string{"-sk", "fixture"},
			WorkDir:  base,
			ExitCode: 0,
		},
		{
			Name:     "combined_shc",
			Args:     []string{"-shc", "fixture"},
			WorkDir:  base,
			ExitCode: 0,
		},
		{
			Name:     "combined_ahd1",
			Args:     []string{"-ahd1", "fixture"},
			WorkDir:  base,
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDuHardLinkDedup tests that hard-linked files are counted only once per
// du invocation, keyed by dev+ino pair.
// (prd009-du R3.1, R3.2, R3.3; AC4)
func TestDuHardLinkDedup(t *testing.T) {
	skipIfMissing(t)

	base := t.TempDir()
	createHardLinkFixture(t, base)

	tests := []testutils.DiffTest{
		{
			Name:     "hard_link_counted_once",
			Args:     []string{"-ak", "hlfix"},
			WorkDir:  base,
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDuErrors tests error handling for nonexistent paths, invalid flags, and
// invalid depth values.
// (prd009-du R4.1, R4.2; AC5)
func TestDuErrors(t *testing.T) {
	skipIfMissing(t)

	tests := []testutils.DiffTest{
		{
			Name:      "nonexistent_path",
			Args:      []string{"-k", "nonexistent_dir_xyz"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{progNameNormalizer},
		},
		{
			Name:      "invalid_flag",
			Args:      []string{"-z"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{errPresenceNormalizer},
		},
		{
			Name:      "invalid_max_depth",
			Args:      []string{"--max-depth=abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{errPresenceNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
