// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd029-comm R1.1–R1.4: three-column comparison
// of two sorted files with byte-for-byte comparison under LC_ALL=C.
// R2.1–R2.4: column suppression flags (-1, -2, -3) with indentation adjustment.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// writeFile creates a file with the given content in dir.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
	return path
}

// setupFiles creates two input files and returns their paths and workdir.
func setupFiles(t *testing.T, content1, content2 string) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	f1 := writeFile(t, dir, "file1", content1)
	f2 := writeFile(t, dir, "file2", content2)
	return f1, f2, dir
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	// Each test creates files and passes absolute paths as args.
	// We build tests dynamically since comm requires file arguments.

	type commTest struct {
		name     string
		args     []string // extra flags before file args
		content1 string
		content2 string
	}

	cases := []commTest{
		// R1.1, R1.2: basic three-column output
		{
			name:     "basic_three_columns",
			content1: "a\nb\nc\n",
			content2: "b\nc\nd\n",
		},
		// R1.1: both files identical
		{
			name:     "identical_files",
			content1: "a\nb\nc\n",
			content2: "a\nb\nc\n",
		},
		// R1.1: no common lines
		{
			name:     "no_common_lines",
			content1: "a\nc\ne\n",
			content2: "b\nd\nf\n",
		},
		// R1.3: file1 exhausted first
		{
			name:     "file1_exhausted_first",
			content1: "a\n",
			content2: "a\nb\nc\n",
		},
		// R1.3: file2 exhausted first
		{
			name:     "file2_exhausted_first",
			content1: "a\nb\nc\n",
			content2: "a\n",
		},
		// R1.3: file1 empty
		{
			name:     "file1_empty",
			content1: "",
			content2: "a\nb\nc\n",
		},
		// R1.3: file2 empty
		{
			name:     "file2_empty",
			content1: "a\nb\nc\n",
			content2: "",
		},
		// R1.3: both files empty
		{
			name:     "both_empty",
			content1: "",
			content2: "",
		},
		// R1.1: single line in each, same
		{
			name:     "single_line_same",
			content1: "hello\n",
			content2: "hello\n",
		},
		// R1.1: single line in each, different
		{
			name:     "single_line_different",
			content1: "apple\n",
			content2: "banana\n",
		},
		// R1.4: case-sensitive comparison (A < a in byte order)
		{
			name:     "case_sensitive",
			content1: "A\na\n",
			content2: "A\na\n",
		},
		// R1.2: interleaved sorted lines
		{
			name:     "interleaved",
			content1: "a\nc\ne\ng\n",
			content2: "b\nd\nf\nh\n",
		},
		// R1.1, R1.2: multiple common and unique
		{
			name:     "mixed_common_unique",
			content1: "a\nb\nd\nf\n",
			content2: "b\nc\nd\ne\n",
		},
		// R1.4: lines with spaces
		{
			name:     "lines_with_spaces",
			content1: "a b\nc d\n",
			content2: "a b\ne f\n",
		},
		// R1.4: trailing newline does not affect comparison
		{
			name:     "no_trailing_newline",
			content1: "a\nb\nc",
			content2: "b\nc\nd",
		},
		// R1.1: long runs of unique in file1 then common
		{
			name:     "long_file1_unique",
			content1: "a\nb\nc\nd\ne\nf\n",
			content2: "f\n",
		},
		// R1.1: long runs of unique in file2 then common
		{
			name:     "long_file2_unique",
			content1: "f\n",
			content2: "a\nb\nc\nd\ne\nf\n",
		},

		// R2.1: -1 suppresses column 1 (file1-only lines)
		{
			name:     "suppress_col1",
			args:     []string{"-1"},
			content1: "a\nb\nc\n",
			content2: "b\nc\nd\n",
		},
		// R2.2: -2 suppresses column 2 (file2-only lines)
		{
			name:     "suppress_col2",
			args:     []string{"-2"},
			content1: "a\nb\nc\n",
			content2: "b\nc\nd\n",
		},
		// R2.3: -3 suppresses column 3 (common lines)
		{
			name:     "suppress_col3",
			args:     []string{"-3"},
			content1: "a\nb\nc\n",
			content2: "b\nc\nd\n",
		},
		// R2.1, R2.2: -12 shows only common lines (col3)
		{
			name:     "suppress_col1_col2",
			args:     []string{"-12"},
			content1: "a\nb\nc\n",
			content2: "b\nc\nd\n",
		},
		// R2.1, R2.3: -13 shows only file2-unique lines (col2)
		{
			name:     "suppress_col1_col3",
			args:     []string{"-13"},
			content1: "a\nb\nc\n",
			content2: "b\nc\nd\n",
		},
		// R2.2, R2.3: -23 shows only file1-unique lines (col1)
		{
			name:     "suppress_col2_col3",
			args:     []string{"-23"},
			content1: "a\nb\nc\n",
			content2: "b\nc\nd\n",
		},
		// R2.3: -123 suppresses all columns, no output
		{
			name:     "suppress_all",
			args:     []string{"-123"},
			content1: "a\nb\nc\n",
			content2: "b\nc\nd\n",
		},
		// R2.4: -1 with no common lines, only col2 output
		{
			name:     "suppress_col1_no_common",
			args:     []string{"-1"},
			content1: "a\nc\n",
			content2: "b\nd\n",
		},
		// R2.4: -2 with identical files, only col3 output
		{
			name:     "suppress_col2_identical",
			args:     []string{"-2"},
			content1: "a\nb\n",
			content2: "a\nb\n",
		},
		// R2.1: -1 with file1 empty (no effect since no col1 lines)
		{
			name:     "suppress_col1_file1_empty",
			args:     []string{"-1"},
			content1: "",
			content2: "a\nb\n",
		},
		// R2.2: -2 with file2 empty (no effect since no col2 lines)
		{
			name:     "suppress_col2_file2_empty",
			args:     []string{"-2"},
			content1: "a\nb\n",
			content2: "",
		},
		// R2.1, R2.2: separate flags -1 -2
		{
			name:     "suppress_col1_col2_separate",
			args:     []string{"-1", "-2"},
			content1: "a\nb\nc\n",
			content2: "b\nc\nd\n",
		},
		// R2.4: -3 indentation shift on interleaved data
		{
			name:     "suppress_col3_interleaved",
			args:     []string{"-3"},
			content1: "a\nc\ne\n",
			content2: "b\nd\nf\n",
		},
	}

	tests := make([]testutils.DiffTest, 0, len(cases))
	for _, tc := range cases {
		f1, f2, _ := setupFiles(t, tc.content1, tc.content2)
		fullArgs := make([]string, 0, len(tc.args)+2)
		fullArgs = append(fullArgs, tc.args...)
		fullArgs = append(fullArgs, f1, f2)
		tests = append(tests, testutils.DiffTest{
			Name: tc.name,
			Args: fullArgs,
		})
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
