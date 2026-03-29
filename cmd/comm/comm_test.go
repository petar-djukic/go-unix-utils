// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// writeTestFiles creates file1.txt and file2.txt in a temp directory.
func writeTestFiles(t *testing.T, content1, content2 string) string {
	t.Helper()
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte(content1), 0o644)
	if err != nil {
		t.Fatalf("writing file1.txt: %v", err)
	}
	err = os.WriteFile(filepath.Join(dir, "file2.txt"), []byte(content2), 0o644)
	if err != nil {
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

	// Setup file pairs for different test scenarios.
	dirBasic := writeTestFiles(t, "a\nb\nc\n", "b\nc\nd\n")
	dirFile1Short := writeTestFiles(t, "a\n", "a\nb\n")
	dirFile2Short := writeTestFiles(t, "a\nb\n", "a\n")
	dirIdentical := writeTestFiles(t, "a\nb\nc\n", "a\nb\nc\n")
	dirNoCommon := writeTestFiles(t, "a\nc\n", "b\nd\n")
	dirFile1Empty := writeTestFiles(t, "", "a\nb\n")
	dirFile2Empty := writeTestFiles(t, "a\nb\n", "")
	dirBothEmpty := writeTestFiles(t, "", "")
	dirNoTrailingNL := writeTestFiles(t, "x", "x")

	tests := []testutils.DiffTest{
		{
			Name:    "R1.1_R1.2_three_column_output",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirBasic,
		},
		{
			Name:    "R1.3_file1_exhausted_first",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirFile1Short,
		},
		{
			Name:    "R1.3_file2_exhausted_first",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirFile2Short,
		},
		{
			Name:    "R1.1_identical_files",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirIdentical,
		},
		{
			Name:    "R1.2_no_common_lines",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirNoCommon,
		},
		{
			Name:    "R1.3_file1_empty",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirFile1Empty,
		},
		{
			Name:    "R1.3_file2_empty",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirFile2Empty,
		},
		{
			Name:    "R1.3_both_empty",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirBothEmpty,
		},
		{
			Name:    "R1.4_no_trailing_newline",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirNoTrailingNL,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
