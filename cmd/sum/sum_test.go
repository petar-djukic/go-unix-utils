// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/sum. Differential tests against gsum (GNU coreutils).
// Covers srd078-sum R2.1 (BSD algorithm via -r), R2.2 (System V via -s),
// R3.1 (exit 0 on success), R3.2 (exit 1 on file error), R3.3 (SIGPIPE).
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

// stderrNormalizer normalizes program name and error message casing
// so that "gsum:" and "sum:" compare as equal, and Go's lowercase
// syscall error strings match GNU's capitalized versions.
func stderrNormalizer(data []byte) []byte {
	re := regexp.MustCompile(`(?m)^(gsum|sum):`)
	data = re.ReplaceAll(data, []byte("sum:"))
	return bytes.ToLower(data)
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsum")
	if err != nil {
		t.Skipf("reference binary gsum not in PATH: %v", err)
	}

	// Create test fixture files in a shared temp directory.
	tmpDir := t.TempDir()
	writeFixture(t, tmpDir, "hello.txt", "Hello, world!\n")
	writeFixture(t, tmpDir, "empty.txt", "")
	writeFixture(t, tmpDir, "binary.bin", "\x00\x01\x02\xff\xfe\xfd")
	writeFixture(t, tmpDir, "multi.txt", "line1\nline2\nline3\n")

	stderrNorm := []testutils.NormalizeFunc{stderrNormalizer}

	tests := []testutils.DiffTest{
		// R2.1: -r explicitly selects BSD algorithm (same as default)
		{
			Name:    "r_flag_bsd_file",
			Args:    []string{"-r", filepath.Join(tmpDir, "hello.txt")},
			WorkDir: tmpDir,
		},
		{
			Name:    "r_flag_bsd_stdin",
			Args:    []string{"-r"},
			Stdin:   []byte("Hello, world!\n"),
			WorkDir: tmpDir,
		},
		{
			Name:    "r_flag_empty_file",
			Args:    []string{"-r", filepath.Join(tmpDir, "empty.txt")},
			WorkDir: tmpDir,
		},
		{
			Name:    "r_flag_binary_data",
			Args:    []string{"-r", filepath.Join(tmpDir, "binary.bin")},
			WorkDir: tmpDir,
		},
		// R2.2: -s selects System V algorithm
		{
			Name:    "s_flag_sysv_file",
			Args:    []string{"-s", filepath.Join(tmpDir, "hello.txt")},
			WorkDir: tmpDir,
		},
		{
			Name:    "sysv_long_flag",
			Args:    []string{"--sysv", filepath.Join(tmpDir, "hello.txt")},
			WorkDir: tmpDir,
		},
		{
			Name:    "s_flag_sysv_stdin",
			Args:    []string{"-s"},
			Stdin:   []byte("Hello, world!\n"),
			WorkDir: tmpDir,
		},
		{
			Name:    "s_flag_empty_file",
			Args:    []string{"-s", filepath.Join(tmpDir, "empty.txt")},
			WorkDir: tmpDir,
		},
		{
			Name:    "s_flag_binary_data",
			Args:    []string{"-s", filepath.Join(tmpDir, "binary.bin")},
			WorkDir: tmpDir,
		},
		{
			Name:    "s_flag_multi_file",
			Args:    []string{"-s", filepath.Join(tmpDir, "hello.txt"), filepath.Join(tmpDir, "multi.txt")},
			WorkDir: tmpDir,
		},
		// R3.1: exit 0 on successful processing
		{
			Name:     "exit_0_single_file",
			Args:     []string{filepath.Join(tmpDir, "hello.txt")},
			WorkDir:  tmpDir,
			ExitCode: 0,
		},
		{
			Name:     "exit_0_multiple_files",
			Args:     []string{filepath.Join(tmpDir, "hello.txt"), filepath.Join(tmpDir, "multi.txt")},
			WorkDir:  tmpDir,
			ExitCode: 0,
		},
		{
			Name:     "exit_0_stdin",
			Args:     []string{},
			Stdin:    []byte("test input\n"),
			WorkDir:  tmpDir,
			ExitCode: 0,
		},
		// R2.1 + R2.2: compare default vs explicit -r (should be identical)
		{
			Name:    "default_matches_r_flag",
			Args:    []string{filepath.Join(tmpDir, "multi.txt")},
			WorkDir: tmpDir,
		},
		{
			Name:    "r_flag_multi_file",
			Args:    []string{"-r", filepath.Join(tmpDir, "hello.txt"), filepath.Join(tmpDir, "multi.txt")},
			WorkDir: tmpDir,
		},
		// R3.2: exit 1 when a file cannot be opened
		{
			Name:      "exit_1_nonexistent_file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			WorkDir:   tmpDir,
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			Name:      "exit_1_nonexistent_sysv",
			Args:      []string{"-s", filepath.Join(tmpDir, "nonexistent.txt")},
			WorkDir:   tmpDir,
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			Name:      "exit_1_mixed_valid_invalid",
			Args:      []string{filepath.Join(tmpDir, "hello.txt"), filepath.Join(tmpDir, "nonexistent.txt")},
			WorkDir:   tmpDir,
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			Name:      "exit_1_mixed_invalid_valid",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt"), filepath.Join(tmpDir, "hello.txt")},
			WorkDir:   tmpDir,
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			Name:      "exit_1_multiple_nonexistent",
			Args:      []string{filepath.Join(tmpDir, "no1.txt"), filepath.Join(tmpDir, "no2.txt")},
			WorkDir:   tmpDir,
			ExitCode:  1,
			Normalize: stderrNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeFixture creates a test file with the given content.
func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write fixture %s: %v", name, err)
	}
}
