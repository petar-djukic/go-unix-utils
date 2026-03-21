// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd057-mv R1.1–R1.4: basic move, rename,
// multi-file move, and error handling.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// binaryNameNormalizer replaces the binary name/path prefix in error
// messages with "mv" so that "gmv:" and "/path/to/mv:" both become "mv:".
func binaryNameNormalizer(b []byte) []byte {
	// Replace gmv with mv at line start
	re := regexp.MustCompile(`(?m)^gmv:`)
	b = re.ReplaceAll(b, []byte("mv:"))
	// Normalize "Try '...' for more information" lines
	reTry := regexp.MustCompile(`Try '[^']*' for more information\.`)
	b = reTry.ReplaceAll(b, []byte("Try 'mv --help' for more information."))
	return b
}

// errorCaseNormalizer normalizes error message casing differences
// between GNU (capitalized) and Go os package (lowercase).
func errorCaseNormalizer(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("No such file or directory"),
		[]byte("no such file or directory"))
	b = bytes.ReplaceAll(b, []byte("Not a directory"),
		[]byte("not a directory"))
	return b
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skip("reference binary gmv not in PATH")
	}
	normalizers := []testutils.NormalizeFunc{
		binaryNameNormalizer,
		errorCaseNormalizer,
	}
	tests := []testutils.DiffTest{
		{
			Name:      "missing_operand",
			Args:      []string{},
			ExitCode:  1,
			Normalize: normalizers,
		},
		{
			Name:      "missing_dest",
			Args:      []string{"somefile"},
			ExitCode:  1,
			Normalize: normalizers,
		},
		missingSourceTest(t, normalizers),
		targetNotADirTest(t, normalizers),
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// missingSourceTest verifies error when source does not exist.
func missingSourceTest(t *testing.T, normalizers []testutils.NormalizeFunc) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	return testutils.DiffTest{
		Name: "missing_source",
		Args: []string{
			filepath.Join(dir, "nonexistent.txt"),
			filepath.Join(dir, "dst.txt"),
		},
		ExitCode:  1,
		Normalize: normalizers,
	}
}

// targetNotADirTest verifies error when multiple sources and target is not a dir.
func targetNotADirTest(t *testing.T, normalizers []testutils.NormalizeFunc) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	// Create files that neither binary will successfully move,
	// since target (c.txt) is a regular file, not a directory.
	writeTestFile(t, filepath.Join(dir, "a.txt"), "aaa\n")
	writeTestFile(t, filepath.Join(dir, "b.txt"), "bbb\n")
	writeTestFile(t, filepath.Join(dir, "c.txt"), "ccc\n")
	return testutils.DiffTest{
		Name: "target_not_a_dir",
		Args: []string{
			filepath.Join(dir, "a.txt"),
			filepath.Join(dir, "b.txt"),
			filepath.Join(dir, "c.txt"),
		},
		ExitCode:  1,
		Normalize: normalizers,
	}
}

// TestMoveOps tests actual move operations using only the Go binary,
// since mv is destructive and the differential test framework runs
// both binaries sequentially in the same working directory.
func TestMoveOps(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("single_file_rename", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		writeTestFile(t, src, "hello\n")
		runExpectSuccess(t, goBin, src, dst)
		assertFileContent(t, dst, "hello\n")
		assertNotExists(t, src)
	})

	t.Run("move_into_dir", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		subdir := filepath.Join(dir, "target")
		mkdirAll(t, subdir)
		src := filepath.Join(dir, "file.txt")
		writeTestFile(t, src, "content\n")
		runExpectSuccess(t, goBin, src, subdir)
		assertFileContent(t, filepath.Join(subdir, "file.txt"), "content\n")
		assertNotExists(t, src)
	})

	t.Run("multi_file_move_into_dir", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		subdir := filepath.Join(dir, "dest")
		mkdirAll(t, subdir)
		a := filepath.Join(dir, "a.txt")
		b := filepath.Join(dir, "b.txt")
		writeTestFile(t, a, "aaa\n")
		writeTestFile(t, b, "bbb\n")
		runExpectSuccess(t, goBin, a, b, subdir)
		assertFileContent(t, filepath.Join(subdir, "a.txt"), "aaa\n")
		assertFileContent(t, filepath.Join(subdir, "b.txt"), "bbb\n")
		assertNotExists(t, a)
		assertNotExists(t, b)
	})

	t.Run("move_directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		srcDir := filepath.Join(dir, "srcdir")
		mkdirAll(t, srcDir)
		writeTestFile(t, filepath.Join(srcDir, "inner.txt"), "inner\n")
		dstDir := filepath.Join(dir, "dstdir")
		runExpectSuccess(t, goBin, srcDir, dstDir)
		assertFileContent(t, filepath.Join(dstDir, "inner.txt"), "inner\n")
		assertNotExists(t, srcDir)
	})
}

// TestHelp verifies --help exits 0 with usage on stdout.
func TestHelp(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	if !bytes.Contains(out, []byte("Usage:")) {
		t.Fatalf("--help output missing Usage header: %s", out)
	}
}

// TestVersion verifies --version exits 0 with version on stdout.
func TestVersion(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if !bytes.Contains(out, []byte("mv")) {
		t.Fatalf("--version output missing 'mv': %s", out)
	}
}

// runExpectSuccess runs the binary and fails the test if it exits non-zero.
func runExpectSuccess(t *testing.T, bin string, args ...string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected success, got error: %v\noutput: %s", err, out)
	}
}

// assertFileContent reads a file and checks it has the expected content.
func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("file %s: got %q, want %q", path, got, want)
	}
}

// assertNotExists checks that a file does not exist.
func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err == nil {
		t.Fatalf("expected %s to not exist, but it does", path)
	}
}

// writeTestFile creates a file with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeTestFile: %v", err)
	}
}

// mkdirAll creates a directory and all parents.
func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}
}
