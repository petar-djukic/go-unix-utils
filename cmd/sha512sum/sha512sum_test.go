// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd033-sha512sum R1.1-R1.4, R2.1-R2.3: core SHA-512
// digest computation, standard GNU output format, stdin reading, multiple file
// processing with error handling, --check verification mode with OK/FAILED
// status output and summary warnings, and --help/--version flags.
package main

import (
	"crypto/sha512"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes stderr differences between our binary ("sha512sum:")
// and the reference binary ("gsha512sum:"), normalizes the "Try '...' for more
// information" line, and lowercases the error message so platform casing
// differences do not cause false failures.
func stderrNormalizer(b []byte) []byte {
	s := string(b)
	s = strings.ReplaceAll(s, "gsha512sum:", "sha512sum:")
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "Try '") && strings.HasSuffix(line, "' for more information.") {
			line = "Try 'sha512sum --help' for more information."
		} else if idx := strings.LastIndex(line, ": "); idx != -1 {
			line = line[:idx+2] + strings.ToLower(line[idx+2:])
		}
		lines = append(lines, line)
	}
	return []byte(strings.Join(lines, "\n"))
}

// sha512Hex computes the SHA-512 hex digest of data.
func sha512Hex(data []byte) string {
	h := sha512.Sum512(data)
	return fmt.Sprintf("%x", h)
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha512sum")
	if err != nil {
		t.Skipf("reference binary gsha512sum not in PATH: %v", err)
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

	hashA := sha512Hex(contentA)
	hashB := sha512Hex(contentB)

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
		// R1.3: nonexistent file — error to stderr, exit 1.
		{
			Name:      "nonexistent file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R1.1/R1.3: mix of valid and nonexistent files.
		{
			Name:      "valid and nonexistent files",
			Args:      []string{fileA, filepath.Join(tmpDir, "nonexistent.txt"), fileB},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	// --- R2.1-R2.3: check mode tests ---

	// Create a valid checksum file for a.txt and b.txt in GNU format.
	checksumFile := filepath.Join(tmpDir, "checksums.txt")
	checksumContent := hashA + "  " + fileA + "\n" +
		hashB + "  " + fileB + "\n"
	if err := os.WriteFile(checksumFile, []byte(checksumContent), 0o644); err != nil {
		t.Fatalf("writing checksums.txt: %v", err)
	}

	// Create a checksum file with a wrong hash for b.txt.
	badChecksumFile := filepath.Join(tmpDir, "bad_checksums.txt")
	badHash := strings.Repeat("0", 127) + "1"
	badChecksumContent := hashA + "  " + fileA + "\n" +
		badHash + "  " + fileB + "\n"
	if err := os.WriteFile(badChecksumFile, []byte(badChecksumContent), 0o644); err != nil {
		t.Fatalf("writing bad_checksums.txt: %v", err)
	}

	// Create a checksum file with binary mode indicator.
	binaryChecksumFile := filepath.Join(tmpDir, "binary_checksums.txt")
	binaryChecksumContent := hashA + " *" + fileA + "\n"
	if err := os.WriteFile(binaryChecksumFile, []byte(binaryChecksumContent), 0o644); err != nil {
		t.Fatalf("writing binary_checksums.txt: %v", err)
	}

	// Create a checksum file referencing a nonexistent file.
	missingFileChecksumFile := filepath.Join(tmpDir, "missing_checksums.txt")
	emptyHash := sha512Hex([]byte{})
	missingContent := emptyHash + "  " + filepath.Join(tmpDir, "nonexistent.txt") + "\n"
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
		// R2.2/R2.3: --check with one mismatch — exit 1, prints FAILED + warning.
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
		// R2.1/R2.2: --check with missing file — exit 1.
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
