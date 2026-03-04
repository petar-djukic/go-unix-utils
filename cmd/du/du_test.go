// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for du core disk usage reporting.
//
// Implements prd009-du R1.1, R1.2, R1.3, R1.5.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goBinary is the path to the compiled Go du binary. Set by TestMain.
var goBinary string

// refBinary is the path to the GNU gdu reference binary. Set by TestMain.
var refBinary string

// TestMain builds the Go du binary and locates the gdu reference binary.
// D1: skip all tests if gdu is not on PATH.
// D1: build Go du binary into a temporary directory.
func TestMain(m *testing.M) {
	ref, err := exec.LookPath("gdu")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gdu not found on PATH; skipping du differential tests")
		os.Exit(0)
	}
	refBinary = ref

	binDir, err := os.MkdirTemp("", "du-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating bin dir: %v\n", err)
		os.Exit(1)
	}

	goBinary = filepath.Join(binDir, "du")
	cmd := exec.Command("go", "build", "-o", goBinary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building Go du binary: %v\n%s", err, out)
		os.RemoveAll(binDir) // best-effort cleanup
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(binDir) // best-effort cleanup
	os.Exit(code)
}

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", name, err)
	}
}

// normalizeProgramName replaces "gdu: " with "du: " in output so stderr
// from the GNU reference binary and the Go binary can be compared.
func normalizeProgramName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gdu: "), []byte("du: "))
}

// buildDirTree creates a two-level directory tree for testing recursive
// traversal. The tree has the structure:
//
//	root/
//	  file1.txt  (small file)
//	  sub/
//	    file2.txt  (small file)
//	    deep/
//	      file3.txt  (small file)
//
// D3: at least two levels of nesting to exercise walkDir recursion.
func buildDirTree(t *testing.T, root string) {
	t.Helper()
	sub := filepath.Join(root, "sub")
	deep := filepath.Join(sub, "deep")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("creating directory tree: %v", err)
	}
	writeTestFile(t, root, "file1.txt", "hello world\n")
	writeTestFile(t, sub, "file2.txt", "second file content\n")
	writeTestFile(t, deep, "file3.txt", "deep nested file\n")
}

// buildFlatDir creates a single directory containing only files (no
// subdirectories) for testing flat directory traversal.
//
//	root/
//	  a.txt
//	  b.txt
func buildFlatDir(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, root, "a.txt", "alpha\n")
	writeTestFile(t, root, "b.txt", "bravo content here\n")
}

// TestDuFlatDirectory verifies R1.1 and R1.3: du on a directory containing
// only files (no subdirectories) prints a single line for the directory itself.
func TestDuFlatDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	buildFlatDir(t, dir)

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "flat-directory",
			Args:     []string{dir},
			ExitCode: 0,
		},
	})
}

// TestDuRecursiveTraversal verifies R1.1: du recurses into each directory
// and prints accumulated size for each subdirectory and the directory itself.
// Also exercises R1.2 (block unit display) and R1.3 (SIZE\tPATH format)
// since RunDiffTests compares stdout byte-for-byte.
func TestDuRecursiveTraversal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	buildDirTree(t, dir)

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "recursive-directory-tree",
			Args:     []string{dir},
			ExitCode: 0,
		},
	})
}

// TestDuBlockUnits verifies R1.2: both binaries report identical block counts
// for the same directory tree, confirming the Go implementation's 512-to-1024
// block conversion matches the reference binary. The byte-for-byte stdout
// comparison in RunDiffTests confirms block counts agree.
func TestDuBlockUnits(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	buildDirTree(t, dir)

	// Run on a subdirectory to verify block counts at a different tree level.
	sub := filepath.Join(dir, "sub")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "block-units-subdirectory",
			Args:     []string{sub},
			ExitCode: 0,
		},
	})
}

// TestDuOutputFormat verifies R1.3: each output line is "SIZE\tPATH\n" for both
// directory and non-directory file arguments.
// D3: tests both directory and non-directory arguments to exercise both branches
// of duArg.
func TestDuOutputFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	buildDirTree(t, dir)

	filePath := filepath.Join(dir, "file1.txt")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "format-directory-arg",
			Args:     []string{dir},
			ExitCode: 0,
		},
		{
			Name:     "format-non-directory-arg",
			Args:     []string{filePath},
			ExitCode: 0,
		},
	})
}

// TestDuDefaultArgument verifies R1.5: when no arguments are given, du defaults
// to ".". Run both binaries with no arguments inside a temporary directory and
// verify identical output.
func TestDuDefaultArgument(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	buildDirTree(t, dir)

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "default-current-directory",
			WorkDir:  dir,
			ExitCode: 0,
		},
	})
}

// TestDuMultipleArguments verifies R1.5: multiple directory arguments are
// processed in command-line order, each traversed independently.
func TestDuMultipleArguments(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	buildDirTree(t, dir)

	sub := filepath.Join(dir, "sub")
	deep := filepath.Join(dir, "sub", "deep")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "multiple-directory-args",
			Args:     []string{sub, deep},
			ExitCode: 0,
		},
	})
}

// TestDuNonexistentArgument verifies R4.1 and R4.2: du exits 1 for a
// nonexistent path and prints a diagnostic to stderr.
func TestDuNonexistentArgument(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:      "nonexistent-path",
			Args:      []string{filepath.Join(dir, "does-not-exist")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
	})
}
