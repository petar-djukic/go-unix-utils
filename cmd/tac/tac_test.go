// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tac against gtac (GNU coreutils).
// Implements prd021-tac R1.1-R1.4, R4.1-R4.3 test coverage.
package main

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
	refBin, err := exec.LookPath("gtac")
	if err != nil {
		t.Skipf("reference binary gtac not in PATH: %v", err)
	}

	// Create test fixtures in a temp directory.
	tmpDir := t.TempDir()
	writeTestFile(t, tmpDir, "three-lines.txt", "a\nb\nc\n")
	writeTestFile(t, tmpDir, "no-trailing-newline.txt", "a\nb\nc")
	writeTestFile(t, tmpDir, "single-line.txt", "hello\n")
	writeTestFile(t, tmpDir, "empty.txt", "")
	writeTestFile(t, tmpDir, "file1.txt", "x\ny\n")
	writeTestFile(t, tmpDir, "file2.txt", "1\n2\n")

	tests := []testutils.DiffTest{
		// R1.1: reverse lines from stdin with trailing newline.
		{
			Name:  "R1.1_reverse_stdin_trailing_newline",
			Stdin: []byte("a\nb\nc\n"),
		},
		// R1.1: reverse lines from a named file.
		{
			Name:    "R1.1_reverse_named_file",
			Args:    []string{filepath.Join(tmpDir, "three-lines.txt")},
			WorkDir: tmpDir,
		},
		// R1.2: trailing newline is terminator, not separator before empty record.
		// "a\nb\n" reversed → "b\na\n", not "\nb\na".
		{
			Name:  "R1.2_trailing_newline_terminator",
			Stdin: []byte("a\nb\n"),
		},
		// R1.2: no trailing newline preserved.
		// "a\nb\nc" reversed → "c\nb\na" (no trailing newline).
		{
			Name:  "R1.2_no_trailing_newline",
			Stdin: []byte("a\nb\nc"),
		},
		// R1.3: read from stdin when no arguments.
		{
			Name:  "R1.3_stdin_no_args",
			Stdin: []byte("first\nsecond\nthird\n"),
		},
		// R1.3: "-" means stdin.
		{
			Name:  "R1.3_dash_means_stdin",
			Args:  []string{"-"},
			Stdin: []byte("alpha\nbeta\n"),
		},
		// R1.4: multiple files processed independently.
		{
			Name: "R1.4_multiple_files_independent",
			Args: []string{
				filepath.Join(tmpDir, "file1.txt"),
				filepath.Join(tmpDir, "file2.txt"),
			},
			WorkDir: tmpDir,
		},
		// Edge: single line with trailing newline.
		{
			Name:  "single_line_trailing_newline",
			Stdin: []byte("only\n"),
		},
		// Edge: single line without trailing newline.
		{
			Name:  "single_line_no_trailing_newline",
			Stdin: []byte("only"),
		},
		// Edge: empty input.
		{
			Name:  "empty_stdin",
			Stdin: []byte(""),
		},
		// Edge: empty file.
		{
			Name:    "empty_file",
			Args:    []string{filepath.Join(tmpDir, "empty.txt")},
			WorkDir: tmpDir,
		},
		// Edge: input with only newlines.
		{
			Name:  "only_newlines",
			Stdin: []byte("\n\n\n"),
		},
		// Edge: file without trailing newline.
		{
			Name:    "file_no_trailing_newline",
			Args:    []string{filepath.Join(tmpDir, "no-trailing-newline.txt")},
			WorkDir: tmpDir,
		},
		// R3.2: non-existent file exits 1.
		{
			Name:      "nonexistent_file_exit_1",
			Args:      []string{filepath.Join(tmpDir, "does-not-exist.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R3.2: non-existent mixed with existing — exit 1, still outputs
		// reversed content of existing files.
		{
			Name: "nonexistent_mixed",
			Args: []string{
				filepath.Join(tmpDir, "three-lines.txt"),
				filepath.Join(tmpDir, "does-not-exist.txt"),
				filepath.Join(tmpDir, "single-line.txt"),
			},
			WorkDir:   tmpDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", name, err)
	}
}

// normalizeProgramName normalizes error messages for differential comparison.
// GNU tac reports errors as "gtac: file: Error" while our binary uses
// "tac: file: error". This normalizer replaces the program name and lowercases
// the output to eliminate both differences.
func normalizeProgramName(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gtac: "), []byte("tac: "))
	return bytes.ToLower(b)
}
