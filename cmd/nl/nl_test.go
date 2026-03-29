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

// writeTestFile is a helper that writes content to path.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestDiff runs differential tests against the GNU reference binary (gnl).
// Covers prd022-nl R1.1 (default numbering format), R1.2 (empty lines),
// R1.3 (stdin input).
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skipf("reference binary gnl not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R1.1, R1.2: non-empty lines numbered, empty line passed through.
			Name:  "default_body_numbering",
			Stdin: []byte("first\n\nsecond\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.2: all empty lines produce bare newlines.
			Name:  "all_empty_lines",
			Stdin: []byte("\n\n\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: single non-empty line numbered at 1.
			Name:  "single_line",
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.2: consecutive empty lines between content.
			Name:  "consecutive_empty_between",
			Stdin: []byte("a\n\n\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: multiple non-empty lines numbered sequentially.
			Name:  "multiple_lines",
			Stdin: []byte("alpha\nbeta\ngamma\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.3: stdin with no trailing newline.
			Name:  "no_trailing_newline",
			Stdin: []byte("line"),
			Env:   []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFileInput runs differential tests for named file input and
// continuous numbering across multiple files.
// Covers prd022-nl R1.3 (named file reading), R1.4 (continuous numbering).
func TestDiffFileInput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skipf("reference binary gnl not in PATH: %v", err)
	}

	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	writeTestFile(t, f1, "alpha\nbeta\n")
	writeTestFile(t, f2, "gamma\ndelta\n")

	tests := []testutils.DiffTest{
		{
			// R1.3: read from a single named file.
			Name: "single_file_input",
			Args: []string{f1},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R1.4: numbering continues across files without reset.
			Name: "continuous_across_files",
			Args: []string{f1, f2},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
