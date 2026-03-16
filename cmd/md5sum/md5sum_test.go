// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd030-md5sum R1.1-R1.4 and R2.1-R2.4: core MD5 digest
// computation, standard GNU output format, and --check verification mode.
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

	// --- R2.1-R2.4: check mode tests ---

	// Create a valid checksum file for a.txt and b.txt.
	// MD5 of "hello\n" = b1946ac92492d2347c6235b4d2611184
	// MD5 of "world\n" = 591785b794601e212b260e25925636fd
	checksumFile := filepath.Join(tmpDir, "checksums.txt")
	checksumContent := "b1946ac92492d2347c6235b4d2611184  " + fileA + "\n" +
		"591785b794601e212b260e25925636fd  " + fileB + "\n"
	if err := os.WriteFile(checksumFile, []byte(checksumContent), 0o644); err != nil {
		t.Fatalf("writing checksums.txt: %v", err)
	}

	// Create a checksum file with a wrong hash for b.txt.
	badChecksumFile := filepath.Join(tmpDir, "bad_checksums.txt")
	badChecksumContent := "b1946ac92492d2347c6235b4d2611184  " + fileA + "\n" +
		"0000000000000000000000000000dead  " + fileB + "\n"
	if err := os.WriteFile(badChecksumFile, []byte(badChecksumContent), 0o644); err != nil {
		t.Fatalf("writing bad_checksums.txt: %v", err)
	}

	// Create a checksum file with binary mode indicator.
	binaryChecksumFile := filepath.Join(tmpDir, "binary_checksums.txt")
	binaryChecksumContent := "b1946ac92492d2347c6235b4d2611184 *" + fileA + "\n"
	if err := os.WriteFile(binaryChecksumFile, []byte(binaryChecksumContent), 0o644); err != nil {
		t.Fatalf("writing binary_checksums.txt: %v", err)
	}

	// Create a checksum file referencing a nonexistent file.
	missingFileChecksumFile := filepath.Join(tmpDir, "missing_checksums.txt")
	missingContent := "d41d8cd98f00b204e9800998ecf8427e  " + filepath.Join(tmpDir, "nonexistent.txt") + "\n"
	if err := os.WriteFile(missingFileChecksumFile, []byte(missingContent), 0o644); err != nil {
		t.Fatalf("writing missing_checksums.txt: %v", err)
	}

	checkTests := []testutils.DiffTest{
		// R2.1/R2.2: --check with all files matching — exit 0, prints OK lines.
		{
			Name: "check all pass",
			Args: []string{"--check", checksumFile},
		},
		// R2.1/R2.2: -c short flag.
		{
			Name: "check short flag all pass",
			Args: []string{"-c", checksumFile},
		},
		// R2.2/R2.3/R2.4: --check with one mismatch — exit 1, prints FAILED + warning.
		{
			Name:      "check with mismatch",
			Args:      []string{"--check", badChecksumFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R2.1: --check with binary mode indicator in checksum file.
		{
			Name: "check binary mode indicator",
			Args: []string{"--check", binaryChecksumFile},
		},
		// R2.4: --check with missing file — exit 1.
		{
			Name:      "check missing file",
			Args:      []string{"--check", missingFileChecksumFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R2.1: --check reads from stdin via "-" argument.
		{
			Name:  "check from stdin dash",
			Args:  []string{"--check", "-"},
			Stdin: []byte(checksumContent),
		},
	}

	tests = append(tests, checkTests...)

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
