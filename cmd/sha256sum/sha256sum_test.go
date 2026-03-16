// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd032-sha256sum R1.1-R1.4: core SHA-256 digest
// computation, standard GNU output format, stdin reading, multiple file
// processing with error handling.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes stderr differences between our binary ("sha256sum:")
// and the reference binary ("gsha256sum:"), normalizes the "Try '...' for more
// information" line, and lowercases the error message so platform casing
// differences do not cause false failures.
func stderrNormalizer(b []byte) []byte {
	s := string(b)
	s = strings.ReplaceAll(s, "gsha256sum:", "sha256sum:")
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "Try '") && strings.HasSuffix(line, "' for more information.") {
			line = "Try 'sha256sum --help' for more information."
		} else if idx := strings.LastIndex(line, ": "); idx != -1 {
			line = line[:idx+2] + strings.ToLower(line[idx+2:])
		}
		lines = append(lines, line)
	}
	return []byte(strings.Join(lines, "\n"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha256sum")
	if err != nil {
		t.Skipf("reference binary gsha256sum not in PATH: %v", err)
	}

	// Create test files in a temp directory.
	tmpDir := t.TempDir()
	fileA := filepath.Join(tmpDir, "a.txt")
	fileB := filepath.Join(tmpDir, "b.txt")
	emptyFile := filepath.Join(tmpDir, "empty.txt")

	contentA := []byte("hello\n")
	contentB := []byte("world\n")

	if err := os.WriteFile(fileA, contentA, 0o644); err != nil {
		t.Fatalf("writing a.txt: %v", err)
	}
	if err := os.WriteFile(fileB, contentB, 0o644); err != nil {
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
		// R1.1: multiple files sequentially.
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
		// R1.4: nonexistent file — error to stderr, exit 1.
		{
			Name:      "nonexistent file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R1.1/R1.4: mix of valid and nonexistent files.
		{
			Name:      "valid and nonexistent files",
			Args:      []string{fileA, filepath.Join(tmpDir, "nonexistent.txt"), fileB},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
