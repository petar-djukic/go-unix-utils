// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/wc against gwc (GNU wc from Homebrew coreutils).
// Implements: prd005-wc R1.1–R1.4 differential testing.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeBinaryName replaces "gwc:" with "wc:" in stderr so the differential
// test does not fail on the binary name prefix difference.
var normalizeBinaryName testutils.NormalizeFunc = func(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gwc:"), []byte("wc:"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skipf("reference binary gwc not in PATH: %v", err)
	}

	// Create test fixture files in a temp directory.
	tmpDir := t.TempDir()

	// file1: simple two-line file
	file1 := filepath.Join(tmpDir, "file1.txt")
	writeTestFile(t, file1, "foo\nbar baz\n")

	// file2: single line no trailing newline
	file2 := filepath.Join(tmpDir, "file2.txt")
	writeTestFile(t, file2, "hello world")

	// empty: zero-byte file
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	writeTestFile(t, emptyFile, "")

	// binary: contains arbitrary bytes
	binaryFile := filepath.Join(tmpDir, "binary.dat")
	writeTestFileBytes(t, binaryFile, []byte{0x00, 0x01, 0xFF, 0x0A, 0x41, 0x42})

	tests := []testutils.DiffTest{
		// R1.1, R1.2: stdin with default flags
		{
			Name:  "stdin_default",
			Stdin: []byte("foo\nbar baz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: empty stdin
		{
			Name:  "stdin_empty",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: single line no trailing newline
		{
			Name:  "stdin_no_trailing_newline",
			Stdin: []byte("hello world"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: single named file
		{
			Name: "single_file",
			Args: []string{file1},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: multiple named files with total line
		{
			Name: "multi_file",
			Args: []string{file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: "-" as stdin alongside named files
		{
			Name:  "dash_stdin_with_file",
			Args:  []string{"-", file1},
			Stdin: []byte("one two three\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: total line with three files
		{
			Name: "three_files_total",
			Args: []string{file1, file2, emptyFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.2: binary input
		{
			Name:  "binary_stdin",
			Stdin: []byte{0x00, 0x01, 0xFF, 0x0A, 0x41, 0x42},
			Env:   []string{"LC_ALL=C"},
		},
		// R4.3: empty file
		{
			Name: "empty_file",
			Args: []string{emptyFile},
			Env:  []string{"LC_ALL=C"},
		},
		// binary file
		{
			Name: "binary_file",
			Args: []string{binaryFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: stdin with multiple words and lines
		{
			Name:  "stdin_multiline",
			Stdin: []byte("line one\nline two\nline three\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: tab and space mixing
		{
			Name:  "stdin_tabs_spaces",
			Stdin: []byte("word1\tword2  word3\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: dash only (stdin as file arg)
		{
			Name:  "dash_only",
			Args:  []string{"-"},
			Stdin: []byte("a b c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Non-existent file
		{
			Name:      "nonexistent_file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// Non-existent file mixed with valid file
		{
			Name:      "nonexistent_mixed",
			Args:      []string{file1, filepath.Join(tmpDir, "nonexistent.txt")},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
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

// writeTestFileBytes writes raw bytes to path, failing the test on error.
func writeTestFileBytes(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", path, err)
	}
}
