// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for file expectation validation in RunDiffTests (prd001-testutils R5.1–R5.2).

package testutils

import (
	"path/filepath"
	"testing"
)

// fileWriterSource is a mock binary that writes a file named "output.txt" in the
// working directory with the content passed as the first argument.
const fileWriterSource = `package main

import "os"

func main() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}
	if err := os.WriteFile("output.txt", []byte(os.Args[1]), 0o644); err != nil {
		os.Exit(1)
	}
}
`

func TestRunDiffTests_ExpectedFiles_Match(t *testing.T) {
	t.Parallel()

	bin := buildMockBinary(t, fileWriterSource)
	workDir := t.TempDir()

	tests := []DiffTest{
		{
			Name:    "file_content_matches",
			Args:    []string{"hello world"},
			WorkDir: workDir,
			ExpectedFiles: map[string][]byte{
				"output.txt": []byte("hello world"),
			},
		},
	}

	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTests_ExpectedFiles_MultipleFiles(t *testing.T) {
	t.Parallel()

	source := `package main

import "os"

func main() {
	os.WriteFile("file1.txt", []byte("content1"), 0o644)
	os.WriteFile("file2.txt", []byte("content2"), 0o644)
}
`
	bin := buildMockBinary(t, source)
	workDir := t.TempDir()

	tests := []DiffTest{
		{
			Name:    "multiple_files_match",
			WorkDir: workDir,
			ExpectedFiles: map[string][]byte{
				"file1.txt": []byte("content1"),
				"file2.txt": []byte("content2"),
			},
		},
	}

	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTests_ExpectedFiles_EmptyContent(t *testing.T) {
	t.Parallel()

	source := `package main

import "os"

func main() {
	os.WriteFile("empty.txt", []byte{}, 0o644)
}
`
	bin := buildMockBinary(t, source)
	workDir := t.TempDir()

	tests := []DiffTest{
		{
			Name:    "empty_file",
			WorkDir: workDir,
			ExpectedFiles: map[string][]byte{
				"empty.txt": {},
			},
		},
	}

	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTests_ExpectedFiles_NilSkipsCheck(t *testing.T) {
	t.Parallel()

	bin := buildMockBinary(t, echoStdoutSource)

	// When ExpectedFiles is nil, no file checks should happen.
	tests := []DiffTest{
		{
			Name:          "no_file_expectations",
			Args:          []string{"hello"},
			ExpectedFiles: nil,
		},
	}

	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTests_ExpectedFiles_SubdirectoryPath(t *testing.T) {
	t.Parallel()

	source := `package main

import (
	"os"
	"path/filepath"
)

func main() {
	os.MkdirAll("sub/dir", 0o755)
	os.WriteFile(filepath.Join("sub", "dir", "file.txt"), []byte("nested"), 0o644)
}
`
	bin := buildMockBinary(t, source)
	workDir := t.TempDir()

	tests := []DiffTest{
		{
			Name:    "subdirectory_file",
			WorkDir: workDir,
			ExpectedFiles: map[string][]byte{
				filepath.Join("sub", "dir", "file.txt"): []byte("nested"),
			},
		},
	}

	RunDiffTests(t, bin, bin, tests)
}
