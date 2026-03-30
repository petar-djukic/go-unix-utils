// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/chmod against gchmod (GNU coreutils).
//
// Traces: prd089-chmod R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// setupFile creates a file with specified permissions in dir.
func setupFile(t *testing.T, dir, name string, perm os.FileMode) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("test\n"), perm); err != nil {
		t.Fatalf("setup: write %s: %v", name, err)
	}
}

// setupDirTree creates a nested directory tree for recursive tests.
func setupDirTree(t *testing.T, dir string) {
	t.Helper()
	sub := filepath.Join(dir, "testdir", "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}
	setupFile(t, filepath.Join(dir, "testdir"), "a.txt", 0o644)
	setupFile(t, sub, "b.txt", 0o644)
}

// makeWorkDir creates a temp dir with a single testfile at perm.
func makeWorkDir(t *testing.T, perm os.FileMode) string {
	t.Helper()
	dir := t.TempDir()
	setupFile(t, dir, "testfile", perm)
	return dir
}

// makeMultiWorkDir creates a temp dir with two files.
func makeMultiWorkDir(t *testing.T, perm os.FileMode) string {
	t.Helper()
	dir := t.TempDir()
	setupFile(t, dir, "file1", perm)
	setupFile(t, dir, "file2", perm)
	return dir
}

// makeRecursiveWorkDir creates a temp dir with a nested directory tree.
func makeRecursiveWorkDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	setupDirTree(t, dir)
	return dir
}

// TestDiff runs differential tests that produce no stdout output, so the
// shared-workdir issue (ref binary modifies state before Go binary) is harmless.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gchmod")
	if err != nil {
		t.Skip("reference binary gchmod not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1: octal mode
		{
			Name:    "octal_755",
			Args:    []string{"755", "testfile"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeWorkDir(t, 0o644),
		},
		// R1.2: symbolic mode
		{
			Name:    "symbolic_u_plus_x",
			Args:    []string{"u+x", "testfile"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeWorkDir(t, 0o644),
		},
		// R1.2: comma-separated symbolic clauses
		{
			Name:    "symbolic_comma",
			Args:    []string{"u+x,go-w", "testfile"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeWorkDir(t, 0o666),
		},
		// R1.3: multiple files
		{
			Name:    "multiple_files",
			Args:    []string{"755", "file1", "file2"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeMultiWorkDir(t, 0o644),
		},
		// R2.1: recursive mode change (no verbose, so idempotent)
		{
			Name:    "recursive_octal",
			Args:    []string{"-R", "755", "testdir"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeRecursiveWorkDir(t),
		},
		// R2.4: silent mode suppresses errors on nonexistent file
		{
			Name:     "silent_nonexistent",
			Args:     []string{"-f", "644", "noexist"},
			Env:      []string{"LC_ALL=C"},
			WorkDir:  t.TempDir(),
			ExitCode: 1,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestVerbose verifies -v diagnostic output against the Go binary directly.
// R2.2: -v prints a diagnostic for every file processed.
func TestVerbose(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	t.Run("mode_changed", func(t *testing.T) {
		dir := makeWorkDir(t, 0o644)
		out := runGoBin(t, goBin, dir, "-v", "755", "testfile")
		want := "mode of 'testfile' changed from 0644 (rw-r--r--) to 0755 (rwxr-xr-x)"
		if strings.TrimSpace(out) != want {
			t.Errorf("got:  %q\nwant: %q", strings.TrimSpace(out), want)
		}
	})

	t.Run("mode_retained", func(t *testing.T) {
		dir := makeWorkDir(t, 0o644)
		out := runGoBin(t, goBin, dir, "-v", "644", "testfile")
		want := "mode of 'testfile' retained as 0644 (rw-r--r--)"
		if strings.TrimSpace(out) != want {
			t.Errorf("got:  %q\nwant: %q", strings.TrimSpace(out), want)
		}
	})
}

// TestChanges verifies -c diagnostic output against the Go binary directly.
// R2.3: -c prints a diagnostic only when mode actually changed.
func TestChanges(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	t.Run("with_change", func(t *testing.T) {
		dir := makeWorkDir(t, 0o644)
		out := runGoBin(t, goBin, dir, "-c", "755", "testfile")
		want := "mode of 'testfile' changed from 0644 (rw-r--r--) to 0755 (rwxr-xr-x)"
		if strings.TrimSpace(out) != want {
			t.Errorf("got:  %q\nwant: %q", strings.TrimSpace(out), want)
		}
	})

	t.Run("no_change", func(t *testing.T) {
		dir := makeWorkDir(t, 0o644)
		out := runGoBin(t, goBin, dir, "-c", "644", "testfile")
		if out != "" {
			t.Errorf("expected no output, got: %q", out)
		}
	})
}

// TestRecursiveVerbose verifies -Rv output against the Go binary directly.
// R2.1 + R2.2: recursive with verbose.
func TestRecursiveVerbose(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := makeRecursiveWorkDir(t)

	out := runGoBin(t, goBin, dir, "-Rv", "755", "testdir")

	// All entries should appear in output
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %q", len(lines), out)
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "mode of '") {
			t.Errorf("unexpected line: %q", line)
		}
	}
}

// TestSilentSuppresses verifies -f suppresses error messages.
// R2.4: -f/--silent/--quiet must suppress most error messages.
func TestSilentSuppresses(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()

	cmd := exec.Command(goBin, "-f", "644", "noexist")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, _ := cmd.CombinedOutput()

	if len(out) != 0 {
		t.Errorf("expected no output with -f, got: %q", string(out))
	}
}

// runGoBin executes the Go binary and returns stdout.
func runGoBin(t *testing.T, bin, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	return string(out)
}
