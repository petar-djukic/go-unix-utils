// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd009-du R1.1–R1.5, R2.1–R2.7.
package main_test

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
	refBin, err := exec.LookPath("gdu")
	if err != nil {
		t.Skipf("reference binary gdu not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		makeRecursionTest(t),
		makeDefaultDirTest(t),
		makeSymlinkTest(t),
		makeFileArgTest(t),
		makeSummaryTest(t),
		makeSummaryMultiArgTest(t),
		makeHumanReadableTest(t),
		makeSummaryHumanTest(t),
		makeAllFilesTest(t),
		makeMultipleArgsTest(t),
		makeDepthLimitTest(t),
		makeDepthZeroTest(t),
		makeKiloBlocksTest(t),
		makeMegaBlocksTest(t),
		makeGrandTotalTest(t),
		makeGrandTotalHumanTest(t),
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// makeRecursionTest creates a fixture with nested subdirectories.
// Covers R1.1 (recursion), R1.2 (1K blocks), R1.3 (output format).
func makeRecursionTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, "a", "b"))
	writeFile(t, filepath.Join(dir, "a", "f1.txt"), "hello world\n")
	writeFile(t, filepath.Join(dir, "a", "b", "f2.txt"), "nested content\n")
	return testutils.DiffTest{
		Name:    "R1.1_R1.2_R1.3_recursion",
		Args:    []string{"."},
		WorkDir: dir,
	}
}

// makeDefaultDirTest verifies R1.1: no arguments defaults to ".".
func makeDefaultDirTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, "sub"))
	writeFile(t, filepath.Join(dir, "sub", "data.txt"), "content\n")
	return testutils.DiffTest{
		Name:    "R1.1_default_current_dir",
		Args:    nil,
		WorkDir: dir,
	}
}

// makeSymlinkTest verifies R1.4: symlinks are not followed during traversal.
// A symlink to a directory inside the fixture must not be recursed into.
func makeSymlinkTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, "real"))
	writeFile(t, filepath.Join(dir, "real", "data.txt"), "some data here\n")
	if err := os.Symlink("real", filepath.Join(dir, "link")); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}
	return testutils.DiffTest{
		Name:    "R1.4_no_follow_symlinks",
		Args:    []string{"."},
		WorkDir: dir,
	}
}

// makeFileArgTest verifies that a file given as a direct argument prints
// its size (R1.1 basic argument handling).
func makeFileArgTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "myfile.txt"), "file content here\n")
	return testutils.DiffTest{
		Name:    "R1.1_file_argument",
		Args:    []string{"myfile.txt"},
		WorkDir: dir,
	}
}

// makeSummaryTest verifies R2.2: -s prints only total per argument.
func makeSummaryTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, "a", "b"))
	writeFile(t, filepath.Join(dir, "a", "f1.txt"), "hello world\n")
	writeFile(t, filepath.Join(dir, "a", "b", "f2.txt"), "nested content\n")
	return testutils.DiffTest{
		Name:    "R2.2_summary_mode",
		Args:    []string{"-s", "."},
		WorkDir: dir,
	}
}

// makeSummaryMultiArgTest verifies R2.2 + R1.5: -s with multiple arguments.
func makeSummaryMultiArgTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, "x"))
	mkdirAll(t, filepath.Join(dir, "y"))
	writeFile(t, filepath.Join(dir, "x", "a.txt"), "hello\n")
	writeFile(t, filepath.Join(dir, "y", "b.txt"), "world\n")
	return testutils.DiffTest{
		Name:    "R2.2_summary_multiple_args",
		Args:    []string{"-s", "x", "y"},
		WorkDir: dir,
	}
}

// makeHumanReadableTest verifies R2.1: -h formats sizes with binary suffixes.
func makeHumanReadableTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, "sub"))
	writeFile(t, filepath.Join(dir, "sub", "data.txt"), "content data here\n")
	return testutils.DiffTest{
		Name:    "R2.1_human_readable",
		Args:    []string{"-h", "."},
		WorkDir: dir,
	}
}

// makeSummaryHumanTest verifies R2.1 + R2.2: -s -h combined.
func makeSummaryHumanTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, "sub"))
	writeFile(t, filepath.Join(dir, "sub", "data.txt"), "content\n")
	return testutils.DiffTest{
		Name:    "R2.1_R2.2_summary_human",
		Args:    []string{"-s", "-h", "."},
		WorkDir: dir,
	}
}

// makeAllFilesTest verifies R2.3: -a prints entries for all files.
// Each directory has at most one entry to avoid readdir order differences
// between Go (sorted) and GNU du (filesystem order).
func makeAllFilesTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, "sub"))
	writeFile(t, filepath.Join(dir, "sub", "file.txt"), "alpha content\n")
	return testutils.DiffTest{
		Name:    "R2.3_all_files",
		Args:    []string{"-a", "."},
		WorkDir: dir,
	}
}

// makeMultipleArgsTest verifies R1.5: multiple directory arguments.
func makeMultipleArgsTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, "d1"))
	mkdirAll(t, filepath.Join(dir, "d2"))
	writeFile(t, filepath.Join(dir, "d1", "a.txt"), "aaa\n")
	writeFile(t, filepath.Join(dir, "d2", "b.txt"), "bbb\n")
	return testutils.DiffTest{
		Name:    "R1.5_multiple_arguments",
		Args:    []string{"d1", "d2"},
		WorkDir: dir,
	}
}

// makeDepthLimitTest verifies R2.4: -d 1 limits output to depth 1.
// Depth 0 is the argument itself, depth 1 is its immediate children.
func makeDepthLimitTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, "a", "b"))
	writeFile(t, filepath.Join(dir, "a", "f1.txt"), "hello world\n")
	writeFile(t, filepath.Join(dir, "a", "b", "f2.txt"), "nested content\n")
	return testutils.DiffTest{
		Name:    "R2.4_max_depth_1",
		Args:    []string{"-d", "1", "."},
		WorkDir: dir,
	}
}

// makeDepthZeroTest verifies R2.4: -d 0 is equivalent to -s.
func makeDepthZeroTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, "sub"))
	writeFile(t, filepath.Join(dir, "sub", "data.txt"), "content\n")
	return testutils.DiffTest{
		Name:    "R2.4_max_depth_0",
		Args:    []string{"-d", "0", "."},
		WorkDir: dir,
	}
}

// makeKiloBlocksTest verifies R2.5: -k is accepted without error.
// Output should be identical to default since 1K is already the default.
func makeKiloBlocksTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, "sub"))
	writeFile(t, filepath.Join(dir, "sub", "data.txt"), "content\n")
	return testutils.DiffTest{
		Name:    "R2.5_kilo_blocks",
		Args:    []string{"-k", "."},
		WorkDir: dir,
	}
}

// makeMegaBlocksTest verifies R2.6: -m reports sizes in 1M blocks.
func makeMegaBlocksTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, "sub"))
	writeFile(t, filepath.Join(dir, "sub", "data.txt"), "content\n")
	return testutils.DiffTest{
		Name:    "R2.6_mega_blocks",
		Args:    []string{"-m", "."},
		WorkDir: dir,
	}
}

// makeGrandTotalTest verifies R2.7: -c prints a grand total line.
func makeGrandTotalTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, "d1"))
	mkdirAll(t, filepath.Join(dir, "d2"))
	writeFile(t, filepath.Join(dir, "d1", "a.txt"), "aaa\n")
	writeFile(t, filepath.Join(dir, "d2", "b.txt"), "bbb\n")
	return testutils.DiffTest{
		Name:    "R2.7_grand_total",
		Args:    []string{"-c", "d1", "d2"},
		WorkDir: dir,
	}
}

// makeGrandTotalHumanTest verifies R2.7 + R2.1: -c -h combined.
func makeGrandTotalHumanTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, "d1"))
	mkdirAll(t, filepath.Join(dir, "d2"))
	writeFile(t, filepath.Join(dir, "d1", "a.txt"), "aaa\n")
	writeFile(t, filepath.Join(dir, "d2", "b.txt"), "bbb\n")
	return testutils.DiffTest{
		Name:    "R2.7_R2.1_grand_total_human",
		Args:    []string{"-c", "-h", "d1", "d2"},
		WorkDir: dir,
	}
}

// mkdirAll creates a directory and its parents.
func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

// writeFile creates a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
