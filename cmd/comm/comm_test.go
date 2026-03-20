// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd029-comm R1.1–R1.4: three-column comparison
// of two sorted files with byte-for-byte comparison under LC_ALL=C.
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
	}

	tests := make([]testutils.DiffTest, 0, len(cases))
	for _, tc := range cases {
		f1, f2, _ := setupFiles(t, tc.content1, tc.content2)
		tests = append(tests, testutils.DiffTest{
			Name: tc.name,
			Args: []string{f1, f2},
		})
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
