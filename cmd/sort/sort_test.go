// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sort against gsort (GNU coreutils).
//
// Covers prd053-sort R1.1, R1.2, R1.3, R1.4.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsort")
	if err != nil {
		t.Skip("reference binary gsort not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1: default lexicographic sort by byte value
		{
			Name:  "R1.1_default_lexicographic",
			Stdin: []byte("banana\napple\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: already sorted input
		{
			Name:  "R1.1_already_sorted",
			Stdin: []byte("apple\nbanana\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: single line
		{
			Name:  "R1.1_single_line",
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: empty input
		{
			Name:  "R1.1_empty_input",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: input without trailing newline
		{
			Name:  "R1.1_no_trailing_newline",
			Stdin: []byte("banana\napple"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: duplicate lines
		{
			Name:  "R1.1_duplicate_lines",
			Stdin: []byte("b\na\nb\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: numeric strings sorted lexicographically (not numerically)
		{
			Name:  "R1.1_numeric_strings_lexicographic",
			Stdin: []byte("10\n2\n1\n20\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: uppercase vs lowercase (byte-value ordering)
		{
			Name:  "R1.1_case_ordering",
			Stdin: []byte("banana\nApple\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: stdin via no file args (implicit)
		{
			Name:  "R1.2_stdin_implicit",
			Stdin: []byte("c\na\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: stdin via explicit "-"
		{
			Name:  "R1.2_stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("c\na\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: multiple files combined
		{
			Name: "R1.3_multi_file",
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: reverse sort
		{
			Name:  "R1.4_reverse",
			Args:  []string{"-r"},
			Stdin: []byte("apple\nbanana\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: --reverse long option
		{
			Name:  "R1.4_reverse_long",
			Args:  []string{"--reverse"},
			Stdin: []byte("apple\nbanana\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: reverse with duplicates
		{
			Name:  "R1.4_reverse_duplicates",
			Args:  []string{"-r"},
			Stdin: []byte("b\na\nb\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	setupMultiFileTest(t, tests)
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// setupMultiFileTest creates temp files for the R1.3 multi-file test.
func setupMultiFileTest(t *testing.T, tests []testutils.DiffTest) {
	t.Helper()
	for i := range tests {
		if tests[i].Name != "R1.3_multi_file" {
			continue
		}
		dir := t.TempDir()
		file1 := filepath.Join(dir, "f1.txt")
		file2 := filepath.Join(dir, "f2.txt")
		writeTestFile(t, file1, "cherry\napple\n")
		writeTestFile(t, file2, "banana\ndate\n")
		tests[i].Args = []string{file1, file2}
		tests[i].Stdin = nil
	}
}

// writeTestFile writes content to a file, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeTestFile %s: %v", path, err)
	}
}
