// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/b2sum implementing prd076-b2sum R1.1, R1.2, R1.3, R1.4,
// R2.1, R2.2, R2.3, R3.1, R3.3.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// clearStderr returns a normalizer that blanks stderr so only stdout and
// exit code are compared.
func clearStderr() testutils.NormalizeFunc {
	return func(b []byte) []byte { return nil }
}

// TestDiff runs differential tests against the gb2sum reference binary.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gb2sum")
	if err != nil {
		t.Skip("reference binary gb2sum not in PATH")
	}

	dir := t.TempDir()
	singleFile := filepath.Join(dir, "hello.txt")
	writeTestFile(t, singleFile, "hello world\n")

	multiFile1 := filepath.Join(dir, "a.txt")
	writeTestFile(t, multiFile1, "aaa\n")
	multiFile2 := filepath.Join(dir, "b.txt")
	writeTestFile(t, multiFile2, "bbb\n")

	tests := []testutils.DiffTest{
		// R1.1: single file digest in GNU text mode.
		{
			Name:     "single_file",
			Args:     []string{singleFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.2: stdin digest when no files given.
		{
			Name:     "stdin_digest",
			Args:     []string{},
			Stdin:    []byte("abc"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.2: explicit "-" reads stdin.
		{
			Name:     "dash_reads_stdin",
			Args:     []string{"-"},
			Stdin:    []byte("test input\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: multiple files produce one line each.
		{
			Name:     "multiple_files",
			Args:     []string{multiFile1, multiFile2},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.1: -b binary mode uses "HASH *FILENAME" format.
		{
			Name:     "binary_mode",
			Args:     []string{"-b", singleFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.1: -t text mode (default) uses "HASH  FILENAME" format.
		{
			Name:     "text_mode_explicit",
			Args:     []string{"-t", singleFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.3: --tag outputs BSD format "BLAKE2b (FILENAME) = HASH".
		{
			Name:     "tag_format",
			Args:     []string{"--tag", singleFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.3: --tag with -b still uses BSD tag format.
		{
			Name:     "tag_with_binary",
			Args:     []string{"--tag", "-b", singleFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.2: empty stdin produces the BLAKE2b of empty input.
		{
			Name:     "empty_stdin",
			Args:     []string{},
			Stdin:    []byte{},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.3: --length=256 produces a 64-char hex digest.
		{
			Name:     "length_256",
			Args:     []string{"--length=256"},
			Stdin:    []byte("hello\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.3: --length=256 with --tag includes bit length in tag name.
		{
			Name:     "length_256_tag",
			Args:     []string{"--length=256", "--tag"},
			Stdin:    []byte("hello\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCheck tests --check mode against the reference binary.
//
// R2.1: -c reads checksum file and verifies entries.
// R2.2: prints OK/FAILED for each entry.
func TestDiffCheck(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gb2sum")
	if err != nil {
		t.Skip("reference binary gb2sum not in PATH")
	}

	dir := t.TempDir()
	dataFile := filepath.Join(dir, "data.txt")
	writeTestFile(t, dataFile, "check me\n")

	checksumFile := filepath.Join(dir, "checksums.txt")
	createChecksumFile(t, refBin, dataFile, checksumFile)

	tests := []testutils.DiffTest{
		// R2.1, R2.2: -c verifies and prints "FILENAME: OK", exits 0.
		{
			Name:     "check_ok",
			Args:     []string{"-c", checksumFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// --quiet suppresses OK lines.
		{
			Name:     "check_quiet",
			Args:     []string{"-c", "--quiet", checksumFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// --status suppresses all output.
		{
			Name:     "check_status",
			Args:     []string{"-c", "--status", checksumFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCheckFailed tests --check mode with a mismatched checksum.
//
// R2.2: prints FAILED and exits 1.
func TestDiffCheckFailed(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gb2sum")
	if err != nil {
		t.Skip("reference binary gb2sum not in PATH")
	}

	dir := t.TempDir()
	dataFile := filepath.Join(dir, "data.txt")
	writeTestFile(t, dataFile, "original\n")

	checksumFile := filepath.Join(dir, "checksums.txt")
	createChecksumFile(t, refBin, dataFile, checksumFile)
	writeTestFile(t, dataFile, "modified\n")

	tests := []testutils.DiffTest{
		{
			Name:      "check_failed",
			Args:      []string{"-c", checksumFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr()},
		},
		{
			Name:     "check_status_failed",
			Args:     []string{"-c", "--status", checksumFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCheckWarnStrict tests --warn and --strict with malformed lines.
//
// R2.3: --warn prints warnings for malformed lines.
// R2.3: --strict exits non-zero for malformed lines.
func TestDiffCheckWarnStrict(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gb2sum")
	if err != nil {
		t.Skip("reference binary gb2sum not in PATH")
	}

	dir := t.TempDir()
	dataFile := filepath.Join(dir, "data.txt")
	writeTestFile(t, dataFile, "test data\n")

	// Create a checksum file with a valid line plus a malformed line.
	checksumFile := filepath.Join(dir, "mixed.txt")
	createChecksumFileWithMalformed(t, refBin, dataFile, checksumFile)

	tests := []testutils.DiffTest{
		// R2.3: --warn prints warning about malformed line, exits 0 if checksums pass.
		{
			Name:      "check_warn_malformed",
			Args:      []string{"-c", "--warn", checksumFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{clearStderr()},
		},
		// R2.3: --strict exits 1 when malformed lines are present.
		{
			Name:      "check_strict_malformed",
			Args:      []string{"-c", "--strict", checksumFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr()},
		},
		// R2.3: --warn --strict together.
		{
			Name:      "check_warn_strict_malformed",
			Args:      []string{"-c", "--warn", "--strict", checksumFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr()},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCheckTag tests --check with BSD tag format checksum files.
//
// R2.1: --check parses BSD tag format.
func TestDiffCheckTag(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gb2sum")
	if err != nil {
		t.Skip("reference binary gb2sum not in PATH")
	}

	dir := t.TempDir()
	dataFile := filepath.Join(dir, "data.txt")
	writeTestFile(t, dataFile, "tag check\n")

	// Generate a BSD-tag format checksum file using the reference binary.
	tagChecksumFile := filepath.Join(dir, "tag_sums.txt")
	createTagChecksumFile(t, refBin, dataFile, tagChecksumFile)

	tests := []testutils.DiffTest{
		// R2.1: --check can verify BSD tag format checksum files.
		{
			Name:     "check_tag_format",
			Args:     []string{"-c", tagChecksumFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffNonexistentFile tests error handling for missing files.
//
// R1.4: exit 1 on unreadable file.
func TestDiffNonexistentFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gb2sum")
	if err != nil {
		t.Skip("reference binary gb2sum not in PATH")
	}

	nonexistent := filepath.Join(t.TempDir(), "no_such_file.txt")

	tests := []testutils.DiffTest{
		{
			Name:      "nonexistent_file",
			Args:      []string{nonexistent},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr()},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// createChecksumFile runs the reference binary to generate a checksum file.
func createChecksumFile(t *testing.T, refBin, dataFile, checksumFile string) {
	t.Helper()
	cmd := exec.Command(refBin, dataFile)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to generate checksum: %v", err)
	}
	if err := os.WriteFile(checksumFile, out, 0o644); err != nil {
		t.Fatalf("failed to write checksum file: %v", err)
	}
}

// createChecksumFileWithMalformed generates a checksum file with one valid
// line and one malformed line.
func createChecksumFileWithMalformed(t *testing.T, refBin, dataFile, checksumFile string) {
	t.Helper()
	cmd := exec.Command(refBin, dataFile)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to generate checksum: %v", err)
	}
	// Append a malformed line after the valid checksum line.
	content := string(out) + "this is not a valid checksum line\n"
	if err := os.WriteFile(checksumFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write checksum file: %v", err)
	}
}

// createTagChecksumFile generates a BSD-tag format checksum file using --tag.
func createTagChecksumFile(t *testing.T, refBin, dataFile, checksumFile string) {
	t.Helper()
	cmd := exec.Command(refBin, "--tag", dataFile)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to generate tag checksum: %v", err)
	}
	if err := os.WriteFile(checksumFile, out, 0o644); err != nil {
		t.Fatalf("failed to write tag checksum file: %v", err)
	}
}

// writeTestFile creates a file with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
}
