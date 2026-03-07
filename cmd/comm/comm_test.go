// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/comm against the GNU reference binary (gcomm).
//
// Implements prd029-comm acceptance criteria AC1-AC6 via testutils.RunDiffTests.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// writeFiles creates file1.txt and file2.txt in dir with the given content.
func writeFiles(t *testing.T, dir, content1, content2 string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte(content1), 0o644); err != nil {
		t.Fatalf("writing file1.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte(content2), 0o644); err != nil {
		t.Fatalf("writing file2.txt: %v", err)
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	// Standard test files: file1='a\nb\nc\n', file2='b\nc\nd\n'.
	dir := t.TempDir()
	writeFiles(t, dir, "a\nb\nc\n", "b\nc\nd\n")

	tests := []testutils.DiffTest{
		// R1.1, R1.2: Full three-column output. AC1.
		{
			Name:    "comm_three_column_output",
			Args:    []string{"file1.txt", "file2.txt"},
			WorkDir: dir,
		},
		// R2.1: -1 suppresses lines unique to file1.
		{
			Name:    "comm_suppress_col1",
			Args:    []string{"-1", "file1.txt", "file2.txt"},
			WorkDir: dir,
		},
		// R2.2: -2 suppresses lines unique to file2.
		{
			Name:    "comm_suppress_col2",
			Args:    []string{"-2", "file1.txt", "file2.txt"},
			WorkDir: dir,
		},
		// R2.1, R2.2: -12 outputs only common lines. AC2.
		{
			Name:    "comm_common_only",
			Args:    []string{"-12", "file1.txt", "file2.txt"},
			WorkDir: dir,
		},
		// R2.3: -3 outputs only unique lines. AC3.
		{
			Name:    "comm_unique_only",
			Args:    []string{"-3", "file1.txt", "file2.txt"},
			WorkDir: dir,
		},
		// R2: -23 outputs only lines unique to file1.
		{
			Name:    "comm_file1_only",
			Args:    []string{"-23", "file1.txt", "file2.txt"},
			WorkDir: dir,
		},
		// R2: -13 outputs only lines unique to file2.
		{
			Name:    "comm_file2_only",
			Args:    []string{"-13", "file1.txt", "file2.txt"},
			WorkDir: dir,
		},
		// R2.3: -123 suppresses all columns (no output).
		{
			Name:    "comm_suppress_all",
			Args:    []string{"-123", "file1.txt", "file2.txt"},
			WorkDir: dir,
		},
		// R3.4: --output-delimiter=| uses pipe as separator. AC4.
		{
			Name:    "comm_output_delimiter",
			Args:    []string{"--output-delimiter=|", "file1.txt", "file2.txt"},
			WorkDir: dir,
		},
		// R3.4: --output-delimiter with -1 flag.
		{
			Name:    "comm_output_delimiter_suppress1",
			Args:    []string{"--output-delimiter=,", "-1", "file1.txt", "file2.txt"},
			WorkDir: dir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffExhausted tests behavior when one file is shorter than the other.
func TestDiffExhausted(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	// R1.3: file1 exhausted first.
	dir1 := t.TempDir()
	writeFiles(t, dir1, "a\n", "a\nb\n")

	// R1.3: file2 exhausted first.
	dir2 := t.TempDir()
	writeFiles(t, dir2, "a\nb\n", "a\n")

	// Both files identical.
	dir3 := t.TempDir()
	writeFiles(t, dir3, "a\nb\nc\n", "a\nb\nc\n")

	// Both files empty.
	dir4 := t.TempDir()
	writeFiles(t, dir4, "", "")

	// File1 empty.
	dir5 := t.TempDir()
	writeFiles(t, dir5, "", "a\nb\n")

	// File2 empty.
	dir6 := t.TempDir()
	writeFiles(t, dir6, "a\nb\n", "")

	// No overlap.
	dir7 := t.TempDir()
	writeFiles(t, dir7, "a\nc\n", "b\nd\n")

	tests := []testutils.DiffTest{
		{
			Name:    "comm_file1_exhausted_first",
			Args:    []string{"file1.txt", "file2.txt"},
			WorkDir: dir1,
		},
		{
			Name:    "comm_file2_exhausted_first",
			Args:    []string{"file1.txt", "file2.txt"},
			WorkDir: dir2,
		},
		{
			Name:    "comm_identical_files",
			Args:    []string{"file1.txt", "file2.txt"},
			WorkDir: dir3,
		},
		{
			Name:    "comm_both_empty",
			Args:    []string{"file1.txt", "file2.txt"},
			WorkDir: dir4,
		},
		{
			Name:    "comm_file1_empty",
			Args:    []string{"file1.txt", "file2.txt"},
			WorkDir: dir5,
		},
		{
			Name:    "comm_file2_empty",
			Args:    []string{"file1.txt", "file2.txt"},
			WorkDir: dir6,
		},
		{
			Name:    "comm_no_overlap",
			Args:    []string{"file1.txt", "file2.txt"},
			WorkDir: dir7,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffNoCheckOrder tests --nocheck-order with unsorted input.
func TestDiffNoCheckOrder(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	dir := t.TempDir()
	writeFiles(t, dir, "b\na\nc\n", "c\na\nb\n")

	tests := []testutils.DiffTest{
		{
			Name:    "comm_nocheck_order",
			Args:    []string{"--nocheck-order", "file1.txt", "file2.txt"},
			WorkDir: dir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffStdin tests reading from stdin via "-".
func TestDiffStdin(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("b\nc\nd\n"), 0o644); err != nil {
		t.Fatalf("writing file2.txt: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:    "comm_stdin_file1",
			Args:    []string{"-", "file2.txt"},
			Stdin:   []byte("a\nb\nc\n"),
			WorkDir: dir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
