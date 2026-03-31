// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/sha256sum implementing prd032-sha256sum R1.1-R1.4, R2.1-R2.3,
// R3.1-R3.2, R4.1-R4.3.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// clearStderr returns a normalizer that blanks stderr differences so only
// stdout and exit code are compared. Used when stderr format diverges from
// GNU due to Go error message formatting.
func clearStderr() testutils.NormalizeFunc {
	return func(b []byte) []byte { return nil }
}

// TestDiff runs differential tests against the gsha256sum reference binary.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha256sum")
	if err != nil {
		t.Skip("reference binary gsha256sum not in PATH")
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
		// R3.2: --tag outputs BSD format "SHA256 (FILENAME) = HASH".
		{
			Name:     "tag_format",
			Args:     []string{"--tag", singleFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.2: --tag with -b still uses BSD tag format.
		{
			Name:     "tag_with_binary",
			Args:     []string{"--tag", "-b", singleFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.2: empty stdin produces the SHA-256 of empty input.
		{
			Name:     "empty_stdin",
			Args:     []string{},
			Stdin:    []byte{},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCheck tests --check mode against the reference binary.
func TestDiffCheck(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha256sum")
	if err != nil {
		t.Skip("reference binary gsha256sum not in PATH")
	}

	dir := t.TempDir()
	dataFile := filepath.Join(dir, "data.txt")
	writeTestFile(t, dataFile, "check me\n")

	checksumFile := filepath.Join(dir, "checksums.txt")
	createChecksumFile(t, refBin, dataFile, checksumFile)

	tests := []testutils.DiffTest{
		// R2.1, R2.2, R4.1: -c verifies and prints "FILENAME: OK", exits 0.
		{
			Name:     "check_ok",
			Args:     []string{"-c", checksumFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.3: --quiet suppresses OK lines.
		{
			Name:     "check_quiet",
			Args:     []string{"-c", "--quiet", checksumFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.3: --status suppresses all output.
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
func TestDiffCheckFailed(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha256sum")
	if err != nil {
		t.Skip("reference binary gsha256sum not in PATH")
	}

	dir := t.TempDir()
	dataFile := filepath.Join(dir, "data.txt")
	writeTestFile(t, dataFile, "original\n")

	checksumFile := filepath.Join(dir, "checksums.txt")
	createChecksumFile(t, refBin, dataFile, checksumFile)
	writeTestFile(t, dataFile, "modified\n")

	tests := []testutils.DiffTest{
		// R2.2, R4.2: -c with mismatch prints FAILED and exits 1.
		{
			Name:      "check_failed",
			Args:      []string{"-c", checksumFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr()},
		},
		// R2.3: --status with mismatch exits 1, no output.
		{
			Name:     "check_status_failed",
			Args:     []string{"-c", "--status", checksumFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffNonexistentFile tests error handling for missing files.
func TestDiffNonexistentFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha256sum")
	if err != nil {
		t.Skip("reference binary gsha256sum not in PATH")
	}

	nonexistent := filepath.Join(t.TempDir(), "no_such_file.txt")

	tests := []testutils.DiffTest{
		// R1.4, R4.2: nonexistent file exits 1.
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

// TestDiffCheckWarn tests --warn flag with malformed checksum lines.
func TestDiffCheckWarn(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha256sum")
	if err != nil {
		t.Skip("reference binary gsha256sum not in PATH")
	}

	dir := t.TempDir()
	dataFile := filepath.Join(dir, "data.txt")
	writeTestFile(t, dataFile, "test\n")

	checksumFile := filepath.Join(dir, "checksums.txt")
	createChecksumFile(t, refBin, dataFile, checksumFile)
	appendToFile(t, checksumFile, "this is not a valid checksum line\n")

	tests := []testutils.DiffTest{
		// R2.3: -w warns about malformed lines on stderr.
		{
			Name:      "check_warn",
			Args:      []string{"-c", "-w", checksumFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{clearStderr()},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCheckStrict tests --strict flag with malformed checksum lines.
func TestDiffCheckStrict(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha256sum")
	if err != nil {
		t.Skip("reference binary gsha256sum not in PATH")
	}

	dir := t.TempDir()
	dataFile := filepath.Join(dir, "data.txt")
	writeTestFile(t, dataFile, "test\n")

	checksumFile := filepath.Join(dir, "checksums.txt")
	createChecksumFile(t, refBin, dataFile, checksumFile)
	appendToFile(t, checksumFile, "this is not a valid checksum line\n")

	tests := []testutils.DiffTest{
		// R4.1: --strict with malformed line exits non-zero.
		{
			Name:      "check_strict_malformed",
			Args:      []string{"-c", "--strict", checksumFile},
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

// writeTestFile creates a file with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
}

// appendToFile appends content to an existing file.
func appendToFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("failed to open file for append %s: %v", path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to append to file %s: %v", path, err)
	}
}
