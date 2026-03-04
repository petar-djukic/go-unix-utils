// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for ls core listing, sorting, hidden file filtering,
// and error handling.
//
// Implements prd008-ls R1.2, R1.3, R1.4, R6.2.
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

// goBinary is the path to the compiled Go ls binary. Set by TestMain.
var goBinary string

// refBinary is the path to the GNU gls reference binary. Set by TestMain.
var refBinary string

// TestMain builds the Go ls binary and locates the gls reference binary.
// D1: skip all tests if gls is not on PATH.
// D1: build Go ls binary into a temporary directory.
func TestMain(m *testing.M) {
	ref, err := exec.LookPath("gls")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gls not found on PATH; skipping ls differential tests")
		os.Exit(0)
	}
	refBinary = ref

	binDir, err := os.MkdirTemp("", "ls-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating bin dir: %v\n", err)
		os.Exit(1)
	}

	goBinary = filepath.Join(binDir, "ls")
	cmd := exec.Command("go", "build", "-o", goBinary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building Go ls binary: %v\n%s", err, out)
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

// normalizeProgramName replaces "gls: " with "ls: " in output so stderr
// from the GNU reference binary and the Go binary can be compared.
// D2: follows the identical pattern in cmd/cat/cat_test.go.
func normalizeProgramName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gls: "), []byte("ls: "))
}

// TestLsSingleColumnOutput verifies R1.2: ls prints one entry per line in
// single-column output mode. Creates a directory with multiple files and
// compares stdout byte-for-byte via testutils.RunDiffTests.
func TestLsSingleColumnOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "alpha.txt", "a\n")
	writeTestFile(t, dir, "bravo.txt", "b\n")
	writeTestFile(t, dir, "charlie.txt", "c\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "multiple-files-single-column",
			Args:     []string{dir},
			ExitCode: 0,
		},
	})
}

// TestLsSortOrder verifies R1.3: ls sorts entries in C locale byte order.
// Creates entries with mixed-case names that sort differently under
// locale-aware vs byte-order collation. Under LC_ALL=C, uppercase letters
// sort before lowercase (ASCII order).
// D3: LC_ALL=C is set automatically by testutils.buildEnv.
func TestLsSortOrder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "banana", "b\n")
	writeTestFile(t, dir, "Apple", "a\n")
	writeTestFile(t, dir, "cherry", "c\n")
	writeTestFile(t, dir, "Cherry", "C\n")
	writeTestFile(t, dir, "apple", "a2\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "mixed-case-byte-order-sort",
			Args:     []string{dir},
			ExitCode: 0,
		},
	})
}

// TestLsHiddenFilesDefault verifies R1.4: ls excludes entries starting with
// '.' by default.
func TestLsHiddenFilesDefault(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "visible.txt", "v\n")
	writeTestFile(t, dir, ".hidden", "h\n")
	writeTestFile(t, dir, "also-visible.txt", "a\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "default-excludes-dotfiles",
			Args:     []string{dir},
			ExitCode: 0,
		},
	})
}

// TestLsHiddenFilesShowAll verifies R1.4 with -a: ls includes all entries
// including . and .. when -a is given.
func TestLsHiddenFilesShowAll(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "visible.txt", "v\n")
	writeTestFile(t, dir, ".hidden", "h\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "show-all-includes-dot-entries",
			Args:     []string{"-a", dir},
			ExitCode: 0,
		},
	})
}

// TestLsHiddenFilesAlmostAll verifies R1.4 with -A: ls includes dotfiles
// but excludes . and .. when -A is given.
func TestLsHiddenFilesAlmostAll(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "visible.txt", "v\n")
	writeTestFile(t, dir, ".hidden", "h\n")
	writeTestFile(t, dir, ".also-hidden", "a\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "almost-all-excludes-dot-dotdot",
			Args:     []string{"-A", dir},
			ExitCode: 0,
		},
	})
}

// TestLsNonexistentPath verifies R6.2: ls writes a diagnostic to stderr for
// nonexistent paths and exits with code 1.
// D2: normalizeProgramName replaces "gls: " with "ls: " before comparison.
func TestLsNonexistentPath(t *testing.T) {
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

// TestLsMixedValidInvalid verifies R6.2: ls continues processing remaining
// valid arguments after encountering a nonexistent path, and exits with
// code 1.
func TestLsMixedValidInvalid(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "file1.txt", "f1\n")
	writeTestFile(t, dir, "file2.txt", "f2\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:      "nonexistent-then-valid-directory",
			Args:      []string{filepath.Join(dir, "no-such-path"), dir},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
	})
}
