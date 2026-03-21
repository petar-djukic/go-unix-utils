// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd053-sort R1.1–R1.7.
package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsort")
	if err != nil {
		t.Skipf("reference binary gsort not in PATH: %v", err)
	}

	// Create temp files for multi-file tests (R1.3).
	tmpDir := t.TempDir()
	fileA := filepath.Join(tmpDir, "a.txt")
	fileB := filepath.Join(tmpDir, "b.txt")
	writeTestFile(t, fileA, "banana\napple\n")
	writeTestFile(t, fileB, "cherry\ndate\n")

	tests := []testutils.DiffTest{
		// R1.1: default lexicographic sort.
		{
			Name:  "default_sort",
			Stdin: []byte("banana\napple\ncherry\n"),
		},
		// R1.1: already sorted input.
		{
			Name:  "already_sorted",
			Stdin: []byte("apple\nbanana\ncherry\n"),
		},
		// R1.1: single line input.
		{
			Name:  "single_line",
			Stdin: []byte("hello\n"),
		},
		// R1.1: empty input.
		{
			Name:  "empty_input",
			Stdin: []byte(""),
		},
		// R1.1: duplicate lines.
		{
			Name:  "duplicate_lines",
			Stdin: []byte("b\na\nb\na\n"),
		},
		// R1.1: case sensitivity (uppercase before lowercase in C locale).
		{
			Name:  "case_sensitivity",
			Stdin: []byte("b\nA\na\nB\n"),
		},
		// R1.2: read from stdin with no arguments.
		{
			Name:  "stdin_no_args",
			Stdin: []byte("z\ny\nx\n"),
		},
		// R1.2: read from stdin via explicit "-".
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("z\ny\nx\n"),
		},
		// R1.3: multiple input files combined and sorted.
		{
			Name: "multi_file",
			Args: []string{fileA, fileB},
		},
		// R1.3: file combined with stdin via "-".
		{
			Name:  "file_and_stdin",
			Args:  []string{fileA, "-"},
			Stdin: []byte("fig\nelderberry\n"),
		},
		// R1.4: reverse sort.
		{
			Name:  "reverse_sort",
			Args:  []string{"-r"},
			Stdin: []byte("apple\ncherry\nbanana\n"),
		},
		// R1.4: reverse with long flag.
		{
			Name:  "reverse_long_flag",
			Args:  []string{"--reverse"},
			Stdin: []byte("apple\ncherry\nbanana\n"),
		},
		// R1.4: reverse with duplicates.
		{
			Name:  "reverse_duplicates",
			Args:  []string{"-r"},
			Stdin: []byte("b\na\nb\na\n"),
		},
		// R1.1: lines with special characters.
		{
			Name:  "special_chars",
			Stdin: []byte("!\n@\n#\na\n1\n"),
		},
		// R1.1: lines with leading whitespace.
		{
			Name:  "leading_whitespace",
			Stdin: []byte(" b\na\n b\n"),
		},
		// R1.3: multiple files with reverse.
		{
			Name: "multi_file_reverse",
			Args: []string{"-r", fileA, fileB},
		},
		// R1.1: numeric strings sorted lexicographically.
		{
			Name:  "numeric_strings_lexico",
			Stdin: []byte("10\n2\n1\n20\n"),
		},
		// R1.2: stdin with empty lines.
		{
			Name:  "stdin_empty_lines",
			Stdin: []byte("\nb\n\na\n"),
		},
		// R1.5: unique removes duplicates.
		{
			Name:  "unique_basic",
			Args:  []string{"-u"},
			Stdin: []byte("b\na\nb\na\nc\n"),
		},
		// R1.5: unique with all identical lines.
		{
			Name:  "unique_all_same",
			Args:  []string{"-u"},
			Stdin: []byte("a\na\na\n"),
		},
		// R1.5: unique with no duplicates.
		{
			Name:  "unique_no_dupes",
			Args:  []string{"-u"},
			Stdin: []byte("c\nb\na\n"),
		},
		// R1.5: unique with long flag.
		{
			Name:  "unique_long_flag",
			Args:  []string{"--unique"},
			Stdin: []byte("b\na\nb\n"),
		},
		// R1.5: unique combined with reverse.
		{
			Name:  "unique_reverse",
			Args:  []string{"-ru"},
			Stdin: []byte("b\na\nb\na\nc\n"),
		},
		// R1.5: unique with empty input.
		{
			Name:  "unique_empty",
			Args:  []string{"-u"},
			Stdin: []byte(""),
		},
		// R1.7: stable sort.
		{
			Name:  "stable_basic",
			Args:  []string{"-s"},
			Stdin: []byte("banana\napple\ncherry\n"),
		},
		// R1.7: stable with long flag.
		{
			Name:  "stable_long_flag",
			Args:  []string{"--stable"},
			Stdin: []byte("c\nb\na\n"),
		},
		// R1.7: stable combined with reverse.
		{
			Name:  "stable_reverse",
			Args:  []string{"-sr"},
			Stdin: []byte("banana\napple\ncherry\n"),
		},
		// R1.7: stable combined with unique.
		{
			Name:  "stable_unique",
			Args:  []string{"-su"},
			Stdin: []byte("b\na\nb\na\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffOutput tests -o/--output flag (R1.6).
func TestDiffOutput(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsort")
	if err != nil {
		t.Skipf("reference binary gsort not in PATH: %v", err)
	}

	tmpDir := t.TempDir()
	out1 := filepath.Join(tmpDir, "out1.txt")
	out2 := filepath.Join(tmpDir, "out2.txt")
	out3 := filepath.Join(tmpDir, "out3.txt")

	tests := []testutils.DiffTest{
		// R1.6: -o writes to file instead of stdout.
		{
			Name:    "output_short_flag",
			Args:    []string{"-o", out1},
			Stdin:   []byte("c\na\nb\n"),
			WorkDir: tmpDir,
		},
		// R1.6: --output=FILE long form.
		{
			Name:    "output_long_flag",
			Args:    []string{"--output=" + out2},
			Stdin:   []byte("z\ny\nx\n"),
			WorkDir: tmpDir,
		},
		// R1.6: -o combined with -u.
		{
			Name:    "output_with_unique",
			Args:    []string{"-u", "-o", out3},
			Stdin:   []byte("b\na\nb\na\n"),
			WorkDir: tmpDir,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)

	// Verify file contents produced by the go binary.
	verifyFileContent(t, out1, "a\nb\nc\n")
	verifyFileContent(t, out2, "x\ny\nz\n")
	verifyFileContent(t, out3, "a\nb\n")
}

// TestOutputInPlace tests -o with the same file as input (R1.6).
func TestOutputInPlace(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	dataFile := filepath.Join(tmpDir, "data.txt")
	writeTestFile(t, dataFile, "banana\napple\ncherry\n")

	cmd := exec.Command(goBin, "-o", dataFile, dataFile)
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sort -o inplace failed: %v\noutput: %s", err, out)
	}

	verifyFileContent(t, dataFile, "apple\nbanana\ncherry\n")
}

// writeTestFile writes content to path, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", path, err)
	}
}

// verifyFileContent checks that a file contains the expected content.
func verifyFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("file %s content mismatch:\ngot:  %q\nwant: %q", path, got, want)
	}
}

// normalizeBinaryPrefix replaces "gsort: " with "sort: " in output so that
// stderr error messages from the reference binary match our binary's output,
// and lowercases the result to normalize strerror() capitalization differences.
func normalizeBinaryPrefix(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("gsort: "), []byte("sort: "))
	return bytes.ToLower(data)
}
