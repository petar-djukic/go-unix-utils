// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd021-tac R1.1–R1.4: core reversal behavior.
package main

import (
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

	// R1.1, R1.2: single-file reversal with trailing newline
	singleFile := createTempFile(t, "a\nb\nc\n")
	// R1.1, R1.2: file with no trailing newline
	noTrailingNL := createTempFile(t, "a\nb\nc")
	// R1.4: multi-file reversal
	multiFile1 := createTempFile(t, "x\ny\n")
	multiFile2 := createTempFile(t, "1\n2\n")

	tests := []testutils.DiffTest{
		{
			// R1.1, R1.2: basic reversal with trailing newline
			Name: "single_file_trailing_newline",
			Args: []string{singleFile},
		},
		{
			// R1.1, R1.2: reversal without trailing newline
			Name: "single_file_no_trailing_newline",
			Args: []string{noTrailingNL},
		},
		{
			// R1.3: stdin reversal
			Name: "stdin_reversal",
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R1.3: stdin with "-" argument
			Name: "stdin_dash_arg",
			Args:  []string{"-"},
			Stdin: []byte("foo\nbar\n"),
		},
		{
			// R1.4: multiple files reversed independently
			Name: "multi_file",
			Args: []string{multiFile1, multiFile2},
		},
		{
			// R1.1: empty input
			Name:  "empty_input",
			Stdin: []byte{},
		},
		{
			// R1.1: single line with newline
			Name:  "single_line_with_newline",
			Stdin: []byte("only\n"),
		},
		{
			// R1.1: single line without newline
			Name:  "single_line_no_newline",
			Stdin: []byte("only"),
		},
		{
			// R1.2: multiple blank lines
			Name:  "blank_lines",
			Stdin: []byte("\n\n\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// createTempFile writes content to a temporary file and returns its path.
func createTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	return path
}
