// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/sum implementing prd078-sum R1.1-R1.4, R2.1, R2.2, R3.1, R3.2, R3.3.
package main

import (
	"fmt"
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
// R2.1: -r selects BSD. R3.1: exit 0 on success.
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
		// R2.1: explicit -r flag (BSD default).
		{
			Name:     "explicit_bsd_flag",
			Args:     []string{"-r", singleFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffNonexistentFile tests R1.4, R3.2: error handling for missing files.
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
		// R1.4, R3.2: nonexistent file exits 1.
		{
			Name:      "nonexistent_file",
			Args:      []string{nonexistent},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr()},
		},
		// R1.4, R3.2: nonexistent among valid files still exits 1, valid files processed.
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

// TestDiffSysV runs differential tests for System V mode (-s) against gsum.
//
// R2.2: -s selects System V algorithm. R3.1: exit 0 on success.
func TestDiffSysV(t *testing.T) {
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

	// R2.2: larger file to exercise block count at 512-byte boundary.
	largeFile := filepath.Join(dir, "large.txt")
	writeTestFile(t, largeFile, makeLargeContent(2000))

	tests := []testutils.DiffTest{
		// R2.2: single file System V checksum.
		{
			Name:     "sysv_single_file",
			Args:     []string{"-s", singleFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.2: stdin in System V mode.
		{
			Name:     "sysv_stdin",
			Args:     []string{"-s"},
			Stdin:    []byte("hello\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.2: empty stdin in System V mode.
		{
			Name:     "sysv_empty_stdin",
			Args:     []string{"-s"},
			Stdin:    []byte{},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.2: empty file in System V mode.
		{
			Name:     "sysv_empty_file",
			Args:     []string{"-s", emptyFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.2: multiple files in System V mode.
		{
			Name:     "sysv_multiple_files",
			Args:     []string{"-s", multiA, multiB},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.2: larger file to test 512-byte block counting.
		{
			Name:     "sysv_large_file",
			Args:     []string{"-s", largeFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffSysVNonexistent tests R3.2: exit 1 for missing files in System V mode.
func TestDiffSysVNonexistent(t *testing.T) {
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
		// R3.2: nonexistent file in System V mode exits 1.
		{
			Name:      "sysv_nonexistent_file",
			Args:      []string{"-s", nonexistent},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr()},
		},
		// R3.2: nonexistent among valid files in System V mode.
		{
			Name:      "sysv_nonexistent_with_valid",
			Args:      []string{"-s", existing, nonexistent},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr()},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestSIGPIPE verifies R3.3: sum exits 0 when stdout is closed early (SIGPIPE).
func TestSIGPIPE(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.txt")
	writeTestFile(t, fileA, "aaa\n")
	fileB := filepath.Join(dir, "b.txt")
	writeTestFile(t, fileB, "bbb\n")

	// Pipe sum (multiple files) through head -1 to trigger SIGPIPE
	// on the second write. Expect exit code 0.
	headBin, err := exec.LookPath("head")
	if err != nil {
		t.Skip("head not in PATH")
	}

	cmd := exec.Command("sh", "-c",
		fmt.Sprintf("%q %q %q | %q -1", goBin, fileA, fileB, headBin))
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("R3.3: expected exit 0 on SIGPIPE, got error: %v\noutput: %s", err, out)
	}
}

// writeTestFile creates a file with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
}

// makeLargeContent generates a string of n bytes of repeating characters.
func makeLargeContent(n int) string {
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = byte('A' + (i % 26))
	}
	return string(buf)
}
