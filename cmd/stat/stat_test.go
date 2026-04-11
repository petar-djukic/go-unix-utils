// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/stat against gstat (GNU coreutils).
// Implements srd082 R1.1, R2.1-R2.3, R3.1, R4.2.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinName = "gstat"

// makeNormalizer creates a NormalizeFunc that replaces binary-specific names
// and normalizes syscall error message capitalization.
func makeNormalizer(refBin string) testutils.NormalizeFunc {
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte(programName))
		b = bytes.ReplaceAll(b, []byte(refBinName), []byte(programName))
		b = normalizeSyscallErrors(b)
		return b
	}
}

// normalizeSyscallErrors lowercases known syscall error messages that
// differ in case between C strerror() and Go syscall.Errno.Error().
func normalizeSyscallErrors(b []byte) []byte {
	replacements := []struct{ from, to string }{
		{"No such file or directory", "no such file or directory"},
		{"Not a directory", "not a directory"},
		{"Permission denied", "permission denied"},
		{"Operation not permitted", "operation not permitted"},
	}
	for _, r := range replacements {
		b = bytes.ReplaceAll(b, []byte(r.from), []byte(r.to))
	}
	return b
}

// setupFixtures creates test files in dir for differential testing.
func setupFixtures(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(
		filepath.Join(dir, "hello.txt"),
		[]byte("hello\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Symlink("hello.txt",
		filepath.Join(dir, "link")); err != nil {
		t.Fatalf("setup: %v", err)
	}
}

// TestDiff runs differential tests comparing cmd/stat against gstat.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	workDir := t.TempDir()
	setupFixtures(t, workDir)

	norm := makeNormalizer(refBin)
	norms := []testutils.NormalizeFunc{norm}

	t.Run("default", func(t *testing.T) {
		t.Parallel()
		runDefaultTests(t, goBin, refBin, workDir, norms)
	})
	t.Run("format", func(t *testing.T) {
		t.Parallel()
		runFormatTests(t, goBin, refBin, workDir, norms)
	})
	t.Run("modes", func(t *testing.T) {
		t.Parallel()
		runModeTests(t, goBin, refBin, workDir, norms)
	})
}

// runDefaultTests tests default multi-line stat output.
func runDefaultTests(
	t *testing.T, goBin, refBin, workDir string,
	norms []testutils.NormalizeFunc,
) {
	t.Helper()
	tests := []testutils.DiffTest{
		{
			Name: "regular_file", Args: []string{"hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "directory", Args: []string{"subdir"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "symlink", Args: []string{"link"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "multiple_files",
			Args:    []string{"hello.txt", "subdir"},
			WorkDir: workDir, Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runFormatTests tests -c format string expansion.
func runFormatTests(
	t *testing.T, goBin, refBin, workDir string,
	norms []testutils.NormalizeFunc,
) {
	t.Helper()
	tests := []testutils.DiffTest{
		{
			Name: "size_name",
			Args:    []string{"-c", "%s %n", "hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "perms",
			Args:    []string{"-c", "%a %A", "hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "ids",
			Args:    []string{"-c", "%u %U %g %G", "hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "device_inode",
			Args:    []string{"-c", "%d %D %i %h", "hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "file_type",
			Args:    []string{"-c", "%F", "hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "blocks_size",
			Args:    []string{"-c", "%b %B %o %f", "hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "epoch_times",
			Args:    []string{"-c", "%X %Y %Z %W", "hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "human_times",
			Args:    []string{"-c", "%x", "hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "quoted_name",
			Args:    []string{"-c", "%N", "link"},
			WorkDir: workDir, Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runModeTests tests terse mode, dereference, and error cases.
func runModeTests(
	t *testing.T, goBin, refBin, workDir string,
	norms []testutils.NormalizeFunc,
) {
	t.Helper()
	tests := []testutils.DiffTest{
		{
			Name: "terse", Args: []string{"-t", "hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "dereference", Args: []string{"-L", "link"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name:     "missing_file",
			Args:     []string{"nonexistent"},
			WorkDir:  workDir,
			ExitCode: 1,
			Normalize: norms,
		},
		{
			Name:     "mixed_valid_invalid",
			Args:     []string{"hello.txt", "nonexistent", "subdir"},
			WorkDir:  workDir,
			ExitCode: 1,
			Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
