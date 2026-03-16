// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/du against gdu (GNU coreutils).
// Implements prd009-du R1.1-R1.5, R2.1-R2.8, R3.1-R3.3, R4.1-R4.2, R5.1 test coverage.
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

	// Deeper directory tree for max-depth tests (R2.4).
	deepDir := filepath.Join(tmpDir, "deep")
	mkDir(t, deepDir)
	mkDir(t, filepath.Join(deepDir, "a"))
	mkDir(t, filepath.Join(deepDir, "a", "b"))
	mkDir(t, filepath.Join(deepDir, "a", "b", "c"))
	writeFile(t, filepath.Join(deepDir, "top.txt"), "top level\n")
	writeFile(t, filepath.Join(deepDir, "a", "mid.txt"), "mid level\n")
	writeFile(t, filepath.Join(deepDir, "a", "b", "deep.txt"), "deep level\n")
	writeFile(t, filepath.Join(deepDir, "a", "b", "c", "bottom.txt"), "bottom level\n")

	// R3.3: Cross-argument hard-link dedup fixture.
	// Two directories each containing a hard link to the same file.
	hlCrossDir1 := filepath.Join(tmpDir, "hlcross1")
	hlCrossDir2 := filepath.Join(tmpDir, "hlcross2")
	mkDir(t, hlCrossDir1)
	mkDir(t, hlCrossDir2)
	writeFile(t, filepath.Join(hlCrossDir1, "shared.txt"), "cross-argument hard link dedup test data\n")
	if err := os.Link(
		filepath.Join(hlCrossDir1, "shared.txt"),
		filepath.Join(hlCrossDir2, "shared.txt"),
	); err != nil {
		t.Fatalf("failed to create cross-directory hard link: %v", err)
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
		// R2.4: -d 0 shows only the argument itself (equivalent to -s).
		{
			Name: "R2.4_max_depth_0",
			Args: []string{"-d", "0", deepDir},
		},
		// R2.4: -d 1 shows argument and immediate children.
		{
			Name: "R2.4_max_depth_1",
			Args: []string{"-d", "1", deepDir},
		},
		// R2.4: --max-depth=2 long form.
		{
			Name: "R2.4_max_depth_2_long",
			Args: []string{"--max-depth=2", deepDir},
		},
		// R2.5: -k accepted without error (1024-byte blocks, already default).
		{
			Name: "R2.5_k_flag",
			Args: []string{"-k", basicDir},
		},
		// R2.6: -m reports sizes in 1M blocks.
		{
			Name: "R2.6_m_flag",
			Args: []string{"-m", basicDir},
		},
		// R2.7: -c prints grand total line.
		{
			Name: "R2.7_grand_total",
			Args: []string{"-c", basicDir, emptyDir},
		},
		// R2.7: -c with -s prints summary per arg plus grand total.
		{
			Name: "R2.7_grand_total_summary",
			Args: []string{"-cs", basicDir, emptyDir},
		},
		// R2.8: --apparent-size reports file size instead of block allocation.
		{
			Name: "R2.8_apparent_size",
			Args: []string{"--apparent-size", basicDir},
		},
		// R2.8: --apparent-size with -a shows apparent size for all files.
		{
			Name: "R2.8_apparent_size_all",
			Args: []string{"--apparent-size", "-a", basicDir},
		},
		// R2.8: --apparent-size with -s shows summary apparent size.
		{
			Name: "R2.8_apparent_size_summary",
			Args: []string{"--apparent-size", "-s", basicDir},
		},
		// R2.8: --apparent-size with -c shows apparent-size grand total.
		{
			Name: "R2.8_apparent_size_grand_total",
			Args: []string{"--apparent-size", "-c", basicDir, emptyDir},
		},
		// R3.3: Hard-link dedup across arguments — file counted only once.
		{
			Name: "R3.3_hardlink_dedup_cross_arg",
			Args: []string{"-a", hlCrossDir1, hlCrossDir2},
		},
		// R3.3: Cross-argument dedup visible with -c grand total.
		{
			Name: "R3.3_hardlink_dedup_cross_arg_total",
			Args: []string{"-cs", hlCrossDir1, hlCrossDir2},
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

// TestVersionHelpExitCodes verifies --version, --help, and invalid option
// exit codes via differential testing per R4.1, R4.2, R5.1.
func TestVersionHelpExitCodes(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdu")
	if err != nil {
		t.Skipf("reference binary gdu not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.1: --version prints version info to stdout and exits 0.
		{
			Name:      "R4.1_version_exits_0",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeAllOutput},
		},
		// R4.2: --help prints usage info to stdout and exits 0.
		{
			Name:      "R4.2_help_exits_0",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeAllOutput},
		},
		// R4.2: Invalid long option exits 1.
		{
			Name:      "R4.2_invalid_long_option_exits_1",
			Args:      []string{"--invalid-xyz-option"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeDuOutput, stripTryHelp},
		},
		// R4.2: Invalid short option exits 1.
		{
			Name:      "R4.2_invalid_short_option_exits_1",
			Args:      []string{"-Z"},
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
// Also normalizes full binary paths (e.g., /opt/homebrew/bin/du:) to just "du:".
func normalizeDuOutput(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gdu: "), []byte("du: "))
	// Normalize full path references to the binary (e.g., from getopt error messages).
	b = normalizeBinaryPath(b)
	return bytes.ToLower(b)
}

// normalizeBinaryPath replaces any path ending in /du: with du: so that
// error messages from the reference binary match our output.
func normalizeBinaryPath(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		// Match lines starting with a path to the du binary.
		if idx := bytes.Index(line, []byte("/du: ")); idx >= 0 {
			lines[i] = append([]byte("du: "), line[idx+len("/du: "):]...)
		}
	}
	return bytes.Join(lines, []byte("\n"))
}

// normalizeAllOutput replaces all output with empty bytes so that only
// exit codes are compared. Used for --version and --help where output
// content intentionally differs between implementations.
func normalizeAllOutput(b []byte) []byte {
	return nil
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
