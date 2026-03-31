// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/cksum implementing prd077-cksum R1.1-R1.4, R2.1-R2.3, R3.1-R3.3.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// clearStderr returns a normalizer that blanks all streams so only
// exit codes are compared.
func clearStderr() testutils.NormalizeFunc {
	return func(b []byte) []byte { return nil }
}

// TestDiff runs differential tests for CRC mode against gcksum.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcksum")
	if err != nil {
		t.Skip("reference binary gcksum not in PATH")
	}

	dir := t.TempDir()
	singleFile := filepath.Join(dir, "hello.txt")
	writeTestFile(t, singleFile, "hello world\n")

	multiA := filepath.Join(dir, "a.txt")
	writeTestFile(t, multiA, "aaa\n")
	multiB := filepath.Join(dir, "b.txt")
	writeTestFile(t, multiB, "bbb\n")

	emptyFile := filepath.Join(dir, "empty.txt")
	writeTestFile(t, emptyFile, "")

	tests := []testutils.DiffTest{
		// R1.1: single file CRC checksum with byte count.
		{
			Name:     "single_file",
			Args:     []string{singleFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.2: stdin when no file arguments given.
		{
			Name:     "stdin_no_args",
			Args:     []string{},
			Stdin:    []byte("abc"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.2: empty stdin.
		{
			Name:     "empty_stdin",
			Args:     []string{},
			Stdin:    []byte{},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.3: multiple files in argument order.
		{
			Name:     "multiple_files",
			Args:     []string{multiA, multiB},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: empty file produces CRC of empty input.
		{
			Name:     "empty_file",
			Args:     []string{emptyFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.2: stdin with newline content.
		{
			Name:     "stdin_with_newline",
			Args:     []string{},
			Stdin:    []byte("hello world\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffNonexistentFile tests error handling for missing files.
func TestDiffNonexistentFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcksum")
	if err != nil {
		t.Skip("reference binary gcksum not in PATH")
	}

	nonexistent := filepath.Join(t.TempDir(), "no_such_file.txt")
	existing := filepath.Join(t.TempDir(), "exists.txt")
	writeTestFile(t, existing, "data\n")

	tests := []testutils.DiffTest{
		// R1.4: nonexistent file exits 1.
		{
			Name:      "nonexistent_file",
			Args:      []string{nonexistent},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr()},
		},
		// R1.4: nonexistent among valid files still exits 1, valid files processed.
		{
			Name:      "nonexistent_with_valid",
			Args:      []string{existing, nonexistent},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr()},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffAlgorithm tests --algorithm flag with various hash algorithms.
//
// R2.1: Algorithm selection produces tagged output by default.
// R2.2: --untagged produces GNU format.
// R2.3: --raw produces raw binary digest.
// R3.1: Exit 0 on success.
func TestDiffAlgorithm(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcksum")
	if err != nil {
		t.Skip("reference binary gcksum not in PATH")
	}

	dir := t.TempDir()
	testFile := filepath.Join(dir, "data.txt")
	writeTestFile(t, testFile, "hello world\n")

	tests := []testutils.DiffTest{
		// R2.1: SHA-256 tagged output.
		{
			Name:     "algo_sha256_tagged",
			Args:     []string{"--algorithm=sha256", testFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1: SHA-1 tagged output.
		{
			Name:     "algo_sha1_tagged",
			Args:     []string{"--algorithm=sha1", testFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1: SHA-224 tagged output.
		{
			Name:     "algo_sha224_tagged",
			Args:     []string{"--algorithm=sha224", testFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1: SHA-384 tagged output.
		{
			Name:     "algo_sha384_tagged",
			Args:     []string{"--algorithm=sha384", testFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1: SHA-512 tagged output.
		{
			Name:     "algo_sha512_tagged",
			Args:     []string{"--algorithm=sha512", testFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1: BLAKE2b tagged output.
		{
			Name:     "algo_blake2b_tagged",
			Args:     []string{"--algorithm=blake2b", testFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1: SM3 tagged output.
		{
			Name:     "algo_sm3_tagged",
			Args:     []string{"--algorithm=sm3", testFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1: SHA-256 with stdin.
		{
			Name:     "algo_sha256_stdin",
			Args:     []string{"--algorithm=sha256"},
			Stdin:    []byte("test input\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1: Explicit CRC algorithm.
		{
			Name:     "algo_crc_explicit",
			Args:     []string{"--algorithm=crc", testFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.2: SHA-256 untagged output.
		{
			Name:     "algo_sha256_untagged",
			Args:     []string{"--algorithm=sha256", "--untagged", testFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.3: SHA-256 raw output.
		{
			Name:     "algo_sha256_raw",
			Args:     []string{"--algorithm=sha256", "--raw", testFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffAlgorithmError tests error handling for invalid algorithms.
func TestDiffAlgorithmError(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcksum")
	if err != nil {
		t.Skip("reference binary gcksum not in PATH")
	}

	tests := []testutils.DiffTest{
		// R3.2: invalid algorithm exits 1.
		{
			Name:      "invalid_algorithm",
			Args:      []string{"--algorithm=invalid"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr()},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffExitCodeErrors tests R3.2: exit 1 on file open failure or invalid
// algorithm for both CRC and non-CRC modes.
func TestDiffExitCodeErrors(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcksum")
	if err != nil {
		t.Skip("reference binary gcksum not in PATH")
	}

	nonexistent := filepath.Join(t.TempDir(), "missing.txt")

	tests := []testutils.DiffTest{
		// R3.2: nonexistent file with --algorithm=sha256 exits 1.
		{
			Name:      "sha256_nonexistent_file",
			Args:      []string{"--algorithm=sha256", nonexistent},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr()},
		},
		// R3.2: nonexistent file with --algorithm=blake2b exits 1.
		{
			Name:      "blake2b_nonexistent_file",
			Args:      []string{"--algorithm=blake2b", nonexistent},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr()},
		},
		// R3.2: invalid algorithm exits 1 (--algorithm=bogus).
		{
			Name:      "invalid_algorithm_bogus",
			Args:      []string{"--algorithm=bogus"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr()},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile creates a file with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
}
