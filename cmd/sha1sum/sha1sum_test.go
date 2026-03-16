// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd031-sha1sum R1.1-R1.4, R2.1-R2.3, R3.1-R3.2:
// core SHA-1 digest computation, standard GNU output format, stdin reading,
// multiple file processing with error handling, --help/--version flags,
// --check verification mode with OK/FAILED output and exit code behavior,
// --tag BSD-style output format, and --binary/--text mode flags.
package main

import (
	"crypto/sha1"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes stderr differences between our binary ("sha1sum:")
// and the reference binary ("gsha1sum:"), normalizes the "Try '...' for more
// information" line, and lowercases the error message so platform casing
// differences do not cause false failures.
func stderrNormalizer(b []byte) []byte {
	s := string(b)
	s = strings.ReplaceAll(s, "gsha1sum:", "sha1sum:")
	// Normalize each line: lowercase everything after the last colon to handle
	// platform differences like "No such file" vs "no such file".
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		// Normalize "Try '...' for more information." lines that contain
		// different binary paths.
		if strings.HasPrefix(line, "Try '") && strings.HasSuffix(line, "' for more information.") {
			line = "Try 'sha1sum --help' for more information."
		} else if idx := strings.LastIndex(line, ": "); idx != -1 {
			line = line[:idx+2] + strings.ToLower(line[idx+2:])
		}
		lines = append(lines, line)
	}
	return []byte(strings.Join(lines, "\n"))
}

// sha1Hex computes the SHA-1 hex digest of data.
func sha1Hex(data []byte) string {
	h := sha1.Sum(data)
	return fmt.Sprintf("%x", h)
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha1sum")
	if err != nil {
		t.Skipf("reference binary gsha1sum not in PATH: %v", err)
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

	hashA := sha1Hex(contentA)
	hashB := sha1Hex(contentB)

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

	// --- R3.1: --binary and --text mode flags ---

	tagTests := []testutils.DiffTest{
		// R3.1: --binary outputs "HASH *FILENAME".
		{
			Name: "binary flag",
			Args: []string{"--binary", fileA},
		},
		// R3.1: -b short flag.
		{
			Name: "binary short flag",
			Args: []string{"-b", fileA},
		},
		// R3.1: --text outputs "HASH  FILENAME" (default).
		{
			Name: "text flag",
			Args: []string{"--text", fileA},
		},
		// R3.1: -t short flag.
		{
			Name: "text short flag",
			Args: []string{"-t", fileA},
		},
		// R3.1: --binary with multiple files.
		{
			Name: "binary multiple files",
			Args: []string{"--binary", fileA, fileB},
		},
		// R3.1: --binary with stdin.
		{
			Name:  "binary stdin",
			Args:  []string{"--binary"},
			Stdin: []byte("abc"),
		},
		// R3.2: --tag produces BSD tag format.
		{
			Name: "tag format",
			Args: []string{"--tag", fileA},
		},
		// R3.2: --tag with multiple files.
		{
			Name: "tag multiple files",
			Args: []string{"--tag", fileA, fileB},
		},
		// R3.2: --tag with stdin.
		{
			Name:  "tag stdin",
			Args:  []string{"--tag"},
			Stdin: []byte("abc"),
		},
		// R3.2: --tag with empty file.
		{
			Name: "tag empty file",
			Args: []string{"--tag", emptyFile},
		},
		// R3.2: --tag combined with --text is an error.
		{
			Name:      "tag with text error",
			Args:      []string{"--tag", "--text", fileA},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	tests = append(tests, tagTests...)

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
	badChecksumContent := hashA + "  " + fileA + "\n" +
		"0000000000000000000000000000000000000dead  " + fileB + "\n"
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
	missingContent := "da39a3ee5e6b4b0d3255bfef95601890afd80709  " + filepath.Join(tmpDir, "nonexistent.txt") + "\n"
	if err := os.WriteFile(missingFileChecksumFile, []byte(missingContent), 0o644); err != nil {
		t.Fatalf("writing missing_checksums.txt: %v", err)
	}

	// Create a BSD tag format checksum file for verifying --check parses tag format.
	tagChecksumFile := filepath.Join(tmpDir, "tag_checksums.txt")
	tagChecksumContent := "SHA1 (" + fileA + ") = " + hashA + "\n" +
		"SHA1 (" + fileB + ") = " + hashB + "\n"
	if err := os.WriteFile(tagChecksumFile, []byte(tagChecksumContent), 0o644); err != nil {
		t.Fatalf("writing tag_checksums.txt: %v", err)
	}

	// R3.2: --tag combined with --check is an error (placed here because checksumFile
	// must be created first).
	tests = append(tests, testutils.DiffTest{
		Name:      "tag with check error",
		Args:      []string{"--tag", "--check", checksumFile},
		ExitCode:  1,
		Normalize: []testutils.NormalizeFunc{stderrNormalizer},
	})

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
		// R2.1/R2.2: --check with BSD tag format checksum file.
		{
			Name: "check tag format",
			Args: []string{"--check", tagChecksumFile},
		},
	}

	tests = append(tests, checkTests...)

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
