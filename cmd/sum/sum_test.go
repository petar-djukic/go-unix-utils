// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/sum implementing prd078-sum R1.1-R1.4.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// clearStderr returns a normalizer that blanks stderr so only
// exit codes and stdout are compared.
func clearStderr() testutils.NormalizeFunc {
	return func(b []byte) []byte { return nil }
}

// TestDiff runs differential tests for BSD mode against gsum.
//
// R1.1: BSD checksum on files. R1.2: stdin. R1.3: multiple files.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsum")
	if err != nil {
		t.Skip("reference binary gsum not in PATH")
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
		// R1.1: single file BSD checksum with block count.
		{
			Name:     "single_file_bsd",
			Args:     []string{singleFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.2: stdin when no file arguments given.
		{
			Name:     "stdin_no_args",
			Args:     []string{},
			Stdin:    []byte("hello\n"),
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
		// R1.1: empty file produces checksum of empty input.
		{
			Name:     "empty_file",
			Args:     []string{emptyFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: explicit -r flag (BSD default).
		{
			Name:     "explicit_bsd_flag",
			Args:     []string{"-r", singleFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffNonexistentFile tests R1.4: error handling for missing files.
func TestDiffNonexistentFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsum")
	if err != nil {
		t.Skip("reference binary gsum not in PATH")
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

// writeTestFile creates a file with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
}
