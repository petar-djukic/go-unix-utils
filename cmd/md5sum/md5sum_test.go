// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd030-md5sum R1.1-R1.4: core MD5 digest computation
// and standard GNU output format.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes stderr differences between our binary ("md5sum:")
// and the reference binary ("gmd5sum:"), and lowercases the error message so
// platform casing differences do not cause false failures.
func stderrNormalizer(b []byte) []byte {
	s := string(b)
	s = strings.ReplaceAll(s, "gmd5sum:", "md5sum:")
	// Normalize each line: lowercase everything after the last colon to handle
	// platform differences like "No such file" vs "no such file".
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if idx := strings.LastIndex(line, ": "); idx != -1 {
			line = line[:idx+2] + strings.ToLower(line[idx+2:])
		}
		lines = append(lines, line)
	}
	return []byte(strings.Join(lines, "\n"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmd5sum")
	if err != nil {
		t.Skipf("reference binary gmd5sum not in PATH: %v", err)
	}

	// Create test files in a temp directory for multi-file and single-file tests.
	tmpDir := t.TempDir()
	fileA := filepath.Join(tmpDir, "a.txt")
	fileB := filepath.Join(tmpDir, "b.txt")
	emptyFile := filepath.Join(tmpDir, "empty.txt")

	if err := os.WriteFile(fileA, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("writing a.txt: %v", err)
	}
	if err := os.WriteFile(fileB, []byte("world\n"), 0o644); err != nil {
		t.Fatalf("writing b.txt: %v", err)
	}
	if err := os.WriteFile(emptyFile, []byte{}, 0o644); err != nil {
		t.Fatalf("writing empty.txt: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: single file hash in text mode (default).
		{
			Name: "single file",
			Args: []string{fileA},
		},
		// R1.3: multiple files sequentially.
		{
			Name: "multiple files",
			Args: []string{fileA, fileB},
		},
		// R1.2: stdin with no arguments.
		{
			Name:  "stdin no args",
			Stdin: []byte("abc"),
		},
		// R1.2: stdin via "-" argument.
		{
			Name:  "stdin dash argument",
			Args:  []string{"-"},
			Stdin: []byte("abc"),
		},
		// R1.1: empty file.
		{
			Name: "empty file",
			Args: []string{emptyFile},
		},
		// R1.2: empty stdin.
		{
			Name:  "empty stdin",
			Stdin: []byte{},
		},
		// R1.4/R3.1: --binary flag output indicator.
		{
			Name: "binary flag single file",
			Args: []string{"--binary", fileA},
		},
		// R3.1: -b short flag.
		{
			Name: "binary short flag",
			Args: []string{"-b", fileA},
		},
		// R3.2: --text flag (default behavior, explicit).
		{
			Name: "text flag single file",
			Args: []string{"--text", fileA},
		},
		// R3.2: -t short flag.
		{
			Name: "text short flag",
			Args: []string{"-t", fileA},
		},
		// R1.4/R3.1: binary mode with stdin.
		{
			Name:  "binary mode stdin",
			Args:  []string{"-b"},
			Stdin: []byte("test data\n"),
		},
		// R1.4: nonexistent file — error to stderr, exit 1.
		{
			Name:      "nonexistent file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R1.3/R1.4: mix of valid and nonexistent files.
		{
			Name:      "valid and nonexistent files",
			Args:      []string{fileA, filepath.Join(tmpDir, "nonexistent.txt"), fileB},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R1.3: multiple files with binary flag.
		{
			Name: "multiple files binary",
			Args: []string{"-b", fileA, fileB},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
