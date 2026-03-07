// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/comm against the GNU reference binary (gcomm).
// Implements prd029-comm R1-R4 verification.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeProgramName replaces "gcomm:" with "comm:" in stderr output so that
// error messages from the reference binary match the Go binary's program name.
var normalizeProgramName testutils.NormalizeFunc = func(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gcomm:"), []byte("comm:"))
}

// writeTestFiles creates a temporary directory containing file1.txt and file2.txt
// with the given contents.
func writeTestFiles(t *testing.T, file1Content, file2Content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte(file1Content), 0o644); err != nil {
		t.Fatalf("writing file1.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte(file2Content), 0o644); err != nil {
		t.Fatalf("writing file2.txt: %v", err)
	}
	return dir
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	// Standard test files: file1=a,b,c  file2=b,c,d
	dirStd := writeTestFiles(t, "a\nb\nc\n", "b\nc\nd\n")
	// file1 shorter than file2
	dirShort := writeTestFiles(t, "a\n", "a\nb\n")
	// Empty file1
	dirEmpty1 := writeTestFiles(t, "", "a\nb\n")
	// Empty file2
	dirEmpty2 := writeTestFiles(t, "a\nb\n", "")
	// Both empty
	dirBothEmpty := writeTestFiles(t, "", "")
	// Identical files
	dirIdent := writeTestFiles(t, "a\nb\nc\n", "a\nb\nc\n")
	// Unsorted file1
	dirUnsorted := writeTestFiles(t, "b\na\n", "a\nb\n")
	// For stdin test: only file2 needed in dir
	dirStdin := t.TempDir()
	if err := os.WriteFile(filepath.Join(dirStdin, "file2.txt"), []byte("b\nc\nd\n"), 0o644); err != nil {
		t.Fatalf("writing file2.txt: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1, R1.2: Full three-column output.
		{
			Name:    "three_column_output",
			Args:    []string{"file1.txt", "file2.txt"},
			WorkDir: dirStd,
		},
		// R2.1: -1 suppresses lines unique to file1.
		{
			Name:    "suppress_col1",
			Args:    []string{"-1", "file1.txt", "file2.txt"},
			WorkDir: dirStd,
		},
		// R2.2: -2 suppresses lines unique to file2.
		{
			Name:    "suppress_col2",
			Args:    []string{"-2", "file1.txt", "file2.txt"},
			WorkDir: dirStd,
		},
		// R2.3: -3 suppresses common lines.
		{
			Name:    "suppress_col3",
			Args:    []string{"-3", "file1.txt", "file2.txt"},
			WorkDir: dirStd,
		},
		// R2.1, R2.2: -12 outputs only common lines.
		{
			Name:    "common_only",
			Args:    []string{"-12", "file1.txt", "file2.txt"},
			WorkDir: dirStd,
		},
		// R2.2, R2.3: -23 outputs only lines unique to file1.
		{
			Name:    "unique_file1_only",
			Args:    []string{"-23", "file1.txt", "file2.txt"},
			WorkDir: dirStd,
		},
		// R2.1, R2.3: -13 outputs only lines unique to file2.
		{
			Name:    "unique_file2_only",
			Args:    []string{"-13", "file1.txt", "file2.txt"},
			WorkDir: dirStd,
		},
		// R2.1, R2.2, R2.3: -123 suppresses all columns (no output).
		{
			Name:    "all_suppressed",
			Args:    []string{"-123", "file1.txt", "file2.txt"},
			WorkDir: dirStd,
		},
		// R3.4: --output-delimiter=| replaces tab separator.
		{
			Name:    "output_delimiter",
			Args:    []string{"--output-delimiter=|", "file1.txt", "file2.txt"},
			WorkDir: dirStd,
		},
		// R1.1: stdin via "-" for file1.
		{
			Name:    "stdin_as_file1",
			Args:    []string{"-", "file2.txt"},
			Stdin:   []byte("a\nb\nc\n"),
			WorkDir: dirStdin,
		},
		// R1.3: file1 shorter — remaining file2 lines go to column 2.
		{
			Name:    "file1_shorter",
			Args:    []string{"file1.txt", "file2.txt"},
			WorkDir: dirShort,
		},
		// R1.3: empty file1 — all file2 lines go to column 2.
		{
			Name:    "empty_file1",
			Args:    []string{"file1.txt", "file2.txt"},
			WorkDir: dirEmpty1,
		},
		// R1.3: empty file2 — all file1 lines go to column 1.
		{
			Name:    "empty_file2",
			Args:    []string{"file1.txt", "file2.txt"},
			WorkDir: dirEmpty2,
		},
		// Edge case: both files empty.
		{
			Name:    "both_empty",
			Args:    []string{"file1.txt", "file2.txt"},
			WorkDir: dirBothEmpty,
		},
		// Identical files — all lines go to column 3.
		{
			Name:    "identical_files",
			Args:    []string{"file1.txt", "file2.txt"},
			WorkDir: dirIdent,
		},
		// R3.2: --check-order with unsorted file1 — fatal error.
		{
			Name:      "unsorted_check_order",
			Args:      []string{"--check-order", "file1.txt", "file2.txt"},
			WorkDir:   dirUnsorted,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R3.3: --nocheck-order with unsorted file1 — no error.
		{
			Name:    "unsorted_nocheck_order",
			Args:    []string{"--nocheck-order", "file1.txt", "file2.txt"},
			WorkDir: dirUnsorted,
		},
		// R3.4: --output-delimiter with -1 (column suppression + custom delimiter).
		{
			Name:    "output_delimiter_with_suppress",
			Args:    []string{"-1", "--output-delimiter=:", "file1.txt", "file2.txt"},
			WorkDir: dirStd,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
