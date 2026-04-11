// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sync against gsync (GNU coreutils).
// Implements srd085 R1.1-R1.4, R2.1-R2.3.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinName = "gsync"

// makeNormalizer creates a NormalizeFunc that replaces binary-specific
// names and normalizes syscall error message capitalization.
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
		{"Is a directory", "is a directory"},
	}
	for _, r := range replacements {
		b = bytes.ReplaceAll(b, []byte(r.from), []byte(r.to))
	}
	return b
}

// TestDiff runs differential tests comparing cmd/sync against gsync.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}
	norm := makeNormalizer(refBin)
	norms := []testutils.NormalizeFunc{norm}

	workDir := t.TempDir()
	setupTestFiles(t, workDir)

	tests := []testutils.DiffTest{
		// R1.1: no arguments — global sync(2).
		{
			Name: "no_args", Args: []string{},
			WorkDir: workDir, ExitCode: 0, Normalize: norms,
		},
		// R1.2: per-file fsync.
		{
			Name: "single_file", Args: []string{"testfile1"},
			WorkDir: workDir, ExitCode: 0, Normalize: norms,
		},
		{
			Name: "multiple_files",
			Args:    []string{"testfile1", "testfile2"},
			WorkDir: workDir, ExitCode: 0, Normalize: norms,
		},
		// R1.3: fdatasync via -d flag.
		{
			Name: "data_flag", Args: []string{"-d", "testfile1"},
			WorkDir: workDir, ExitCode: 0, Normalize: norms,
		},
		{
			Name: "data_long_flag",
			Args:    []string{"--data", "testfile1"},
			WorkDir: workDir, ExitCode: 0, Normalize: norms,
		},
		// R1.4: filesystem sync via -f flag.
		{
			Name: "filesystem_flag", Args: []string{"-f", "testfile1"},
			WorkDir: workDir, ExitCode: 0, Normalize: norms,
		},
		{
			Name: "filesystem_long_flag",
			Args:    []string{"--file-system", "testfile1"},
			WorkDir: workDir, ExitCode: 0, Normalize: norms,
		},
		// R1.3: -d without files exits 1.
		{
			Name: "data_no_files", Args: []string{"-d"},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
		// -f without files falls through to global sync(2), exits 0.
		{
			Name: "filesystem_no_files", Args: []string{"-f"},
			WorkDir: workDir, ExitCode: 0, Normalize: norms,
		},
		// R2.2: nonexistent file exits 1.
		{
			Name: "nonexistent_file",
			Args:    []string{"doesnotexist"},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// setupTestFiles creates test fixture files in the work directory.
func setupTestFiles(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{"testfile1", "testfile2"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
			t.Fatalf("setup: write %s: %v", name, err)
		}
	}
}
