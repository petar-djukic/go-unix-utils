// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sha1sum against gsha1sum (GNU coreutils).
// Implements prd031-sha1sum AC1-AC5.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrBinaryNameNormalizer replaces "gsha1sum:" with "sha1sum:" so stderr
// messages from both binaries compare equal.
var stderrBinaryNameNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gsha1sum:"), []byte("sha1sum:"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha1sum")
	if err != nil {
		t.Skipf("reference binary gsha1sum not in PATH: %v", err)
	}

	// Create a shared work directory with test fixtures.
	workDir := t.TempDir()
	writeTestFile(t, workDir, "hello.txt", "hello\n")
	writeTestFile(t, workDir, "empty.txt", "")
	writeTestFile(t, workDir, "multi1.txt", "aaa\n")
	writeTestFile(t, workDir, "multi2.txt", "bbb\n")

	// Valid checksum file for hello.txt (SHA-1 of "hello\n").
	writeTestFile(t, workDir, "checksums.sha1",
		"f572d396fae9206628714fb2ce00f72e94f2258f  hello.txt\n")

	// Invalid checksum file for hello.txt (wrong hash).
	writeTestFile(t, workDir, "bad.sha1",
		"0000000000000000000000000000000000000000  hello.txt\n")

	tests := []testutils.DiffTest{
		{
			Name:    "compute_stdin",
			Args:    []string{},
			Stdin:   []byte("hello\n"),
			WorkDir: workDir,
		},
		{
			Name:    "compute_file",
			Args:    []string{"hello.txt"},
			WorkDir: workDir,
		},
		{
			Name:    "compute_empty_file",
			Args:    []string{"empty.txt"},
			WorkDir: workDir,
		},
		{
			Name:    "compute_multiple_files",
			Args:    []string{"multi1.txt", "multi2.txt"},
			WorkDir: workDir,
		},
		{
			Name:    "binary_mode",
			Args:    []string{"-b", "hello.txt"},
			WorkDir: workDir,
		},
		{
			Name:    "tag_format",
			Args:    []string{"--tag", "hello.txt"},
			WorkDir: workDir,
		},
		{
			Name:    "check_valid",
			Args:    []string{"--check", "checksums.sha1"},
			WorkDir: workDir,
		},
		{
			Name:      "check_failure",
			Args:      []string{"--check", "bad.sha1"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrBinaryNameNormalizer},
		},
		{
			Name:      "nonexistent_file",
			Args:      []string{"no_such_file.txt"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrBinaryNameNormalizer},
		},
		{
			Name:    "stdin_dash",
			Args:    []string{"-"},
			Stdin:   []byte("hello\n"),
			WorkDir: workDir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	if err != nil {
		t.Fatalf("failed to write test file %s: %v", name, err)
	}
}
