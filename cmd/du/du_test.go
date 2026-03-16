// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/du against gdu (GNU coreutils).
// Implements prd009-du R1.1-R1.5, R2.2-R2.3, R3.1-R3.3, R4.1-R4.2 test coverage.
package main

import (
	"bytes"
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

	// Create test fixtures in a temp directory.
	tmpDir := t.TempDir()

	// Basic directory tree.
	basicDir := filepath.Join(tmpDir, "basic")
	mkDir(t, basicDir)
	mkDir(t, filepath.Join(basicDir, "sub"))
	writeFile(t, filepath.Join(basicDir, "file1.txt"), "hello world\n")
	writeFile(t, filepath.Join(basicDir, "sub", "file2.txt"), "test content here\n")

	// Empty directory.
	emptyDir := filepath.Join(tmpDir, "empty")
	mkDir(t, emptyDir)

	// Hard-link fixture: original.txt hard-linked as link.txt.
	hlDir := filepath.Join(tmpDir, "hardlink")
	mkDir(t, hlDir)
	writeFile(t, filepath.Join(hlDir, "original.txt"), "hard link test data for dedup verification\n")
	if err := os.Link(
		filepath.Join(hlDir, "original.txt"),
		filepath.Join(hlDir, "link.txt"),
	); err != nil {
		t.Fatalf("failed to create hard link: %v", err)
	}

	// Symlink fixture: verify symlinks are not followed.
	slDir := filepath.Join(tmpDir, "symlink")
	mkDir(t, slDir)
	mkDir(t, filepath.Join(slDir, "realdir"))
	writeFile(t, filepath.Join(slDir, "file.txt"), "symlink test\n")
	writeFile(t, filepath.Join(slDir, "realdir", "inner.txt"), "inner content\n")
	// Symlink to a file.
	if err := os.Symlink(
		filepath.Join(slDir, "file.txt"),
		filepath.Join(slDir, "filelink"),
	); err != nil {
		t.Fatalf("failed to create file symlink: %v", err)
	}
	// Symlink to a directory — must not be recursed into.
	if err := os.Symlink(
		filepath.Join(slDir, "realdir"),
		filepath.Join(slDir, "dirlink"),
	); err != nil {
		t.Fatalf("failed to create dir symlink: %v", err)
	}

	// Permission denied fixture: subdirectory without read permission.
	noPermDir := filepath.Join(tmpDir, "noperm")
	mkDir(t, noPermDir)
	writeFile(t, filepath.Join(noPermDir, "visible.txt"), "visible\n")
	mkDir(t, filepath.Join(noPermDir, "locked"))
	writeFile(t, filepath.Join(noPermDir, "locked", "secret.txt"), "secret\n")
	if err := os.Chmod(filepath.Join(noPermDir, "locked"), 0o000); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(filepath.Join(noPermDir, "locked"), 0o755) // best-effort restore for cleanup
	})

	tests := []testutils.DiffTest{
		// R1.1: Basic directory traversal in depth-first order.
		{
			Name: "R1.1_basic_directory",
			Args: []string{basicDir},
		},
		// R1.1: Default to current directory when no args given.
		{
			Name:    "R1.1_default_current_dir",
			WorkDir: basicDir,
		},
		// R1.1: Empty directory reports only its own size.
		{
			Name: "R1.1_empty_directory",
			Args: []string{emptyDir},
		},
		// R1.4: Symlinks not followed during traversal.
		{
			Name: "R1.4_symlinks_not_followed",
			Args: []string{slDir},
		},
		// R1.5: Multiple arguments processed in command-line order.
		{
			Name: "R1.5_multiple_arguments",
			Args: []string{basicDir, emptyDir},
		},
		// R2.2: -s summary mode — one line per argument.
		{
			Name: "R2.2_summary_mode",
			Args: []string{"-s", basicDir},
		},
		// R2.2: -s with multiple arguments.
		{
			Name: "R2.2_summary_multiple",
			Args: []string{"-s", basicDir, emptyDir},
		},
		// R2.3: -a all files mode — entries for every file.
		{
			Name: "R2.3_all_files",
			Args: []string{"-a", basicDir},
		},
		// R3.1: Hard-link deduplication — file counted only once.
		{
			Name: "R3.1_hardlink_dedup",
			Args: []string{hlDir},
		},
		// R3.1: Hard-link dedup visible with -a (one link shows 0).
		{
			Name: "R3.1_hardlink_dedup_all",
			Args: []string{"-a", hlDir},
		},
		// R3.1: Hard-link dedup total with -s.
		{
			Name: "R3.1_hardlink_dedup_summary",
			Args: []string{"-s", hlDir},
		},
		// R4.2: Non-existent path — error on stderr, exit 1.
		{
			Name:      "R4.2_nonexistent_path",
			Args:      []string{filepath.Join(tmpDir, "nonexistent")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeDuOutput},
		},
		// R4.2: Non-existent mixed with valid — continue processing.
		{
			Name: "R4.2_nonexistent_mixed",
			Args: []string{
				filepath.Join(tmpDir, "nonexistent"),
				basicDir,
			},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeDuOutput},
		},
		// R4.2: Permission denied on subdirectory.
		{
			Name:      "R4.2_permission_denied",
			Args:      []string{noPermDir},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeDuOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestMutualExclusion verifies that -s and -a together produce an error
// and exit 1, per R1.3.
func TestMutualExclusion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdu")
	if err != nil {
		t.Skipf("reference binary gdu not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:      "R1.3_s_and_a_mutual_exclusion",
			Args:      []string{"-s", "-a", "."},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeDuOutput, stripTryHelp},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// mkDir creates a directory with standard permissions.
func mkDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("failed to create directory %s: %v", path, err)
	}
}

// writeFile creates a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}

// normalizeDuOutput normalizes error messages for differential comparison.
// Replaces the program name (gdu → du) and lowercases output so that
// platform-specific error string capitalization does not cause false failures.
func normalizeDuOutput(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gdu: "), []byte("du: "))
	return bytes.ToLower(b)
}

// stripTryHelp removes GNU's "Try '...' for more information" line that
// our implementation does not emit.
func stripTryHelp(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	var filtered [][]byte
	for _, line := range lines {
		if bytes.HasPrefix(bytes.ToLower(line), []byte("try '")) {
			continue
		}
		filtered = append(filtered, line)
	}
	return bytes.Join(filtered, []byte("\n"))
}
