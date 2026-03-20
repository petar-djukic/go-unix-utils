// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd053-sort R1.1–R1.4.
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
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile writes content to path, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", path, err)
	}
}

// normalizeBinaryPrefix replaces "gsort: " with "sort: " in output so that
// stderr error messages from the reference binary match our binary's output,
// and lowercases the result to normalize strerror() capitalization differences.
func normalizeBinaryPrefix(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("gsort: "), []byte("sort: "))
	return bytes.ToLower(data)
}
