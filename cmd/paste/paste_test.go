// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/paste against the GNU reference binary (gpaste).
// Implements prd027-paste R1.1-R1.4 test coverage: multi-file parallel merge
// with default tab delimiter, stdin via '-' operand with round-robin consumption,
// unequal-length file handling, and single-file passthrough.
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
	refBin, err := exec.LookPath("gpaste")
	if err != nil {
		t.Skipf("reference binary gpaste not in PATH: %v", err)
	}

	// Create temp files for file-input tests.
	tmpDir := t.TempDir()

	fileA := filepath.Join(tmpDir, "a.txt")
	if err := os.WriteFile(fileA, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	fileNums := filepath.Join(tmpDir, "nums.txt")
	if err := os.WriteFile(fileNums, []byte("1\n2\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Unequal-length files for R1.3 testing.
	fileShort := filepath.Join(tmpDir, "short.txt")
	if err := os.WriteFile(fileShort, []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	fileLong := filepath.Join(tmpDir, "long.txt")
	if err := os.WriteFile(fileLong, []byte("1\n2\n3\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	fileThree := filepath.Join(tmpDir, "three.txt")
	if err := os.WriteFile(fileThree, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte{}, 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	singleLine := filepath.Join(tmpDir, "single.txt")
	if err := os.WriteFile(singleLine, []byte("only\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	tests := []testutils.DiffTest{
		// --- R1.1: basic multi-file paste with tab delimiter (AC4) ---
		{
			Name: "two_files_tab_delim",
			Args: []string{fileA, fileNums},
		},
		{
			Name: "three_files_tab_delim",
			Args: []string{fileA, fileNums, fileThree},
		},

		// --- R1.2: default tab delimiter (AC4) ---
		{
			Name: "two_files_default_tab",
			Args: []string{fileA, fileNums},
		},

		// --- R1.3: unequal-length files (AC6) ---
		{
			Name: "unequal_short_first",
			Args: []string{fileShort, fileLong},
		},
		{
			Name: "unequal_long_first",
			Args: []string{fileLong, fileShort},
		},
		{
			Name: "empty_and_nonempty",
			Args: []string{emptyFile, fileA},
		},
		{
			Name: "nonempty_and_empty",
			Args: []string{fileA, emptyFile},
		},
		{
			Name: "three_files_unequal",
			Args: []string{fileShort, fileLong, fileA},
		},

		// --- R1.4: single-file paste / passthrough (AC7) ---
		{
			Name: "single_file_passthrough",
			Args: []string{fileA},
		},
		{
			Name: "single_file_multiline",
			Args: []string{fileThree},
		},

		// --- R1.4: stdin via '-' (AC5) ---
		{
			Name:  "stdin_single_dash",
			Args:  []string{"-"},
			Stdin: []byte("hello\nworld\n"),
		},
		{
			// No files given, reads stdin by default.
			Name:  "stdin_no_args",
			Stdin: []byte("hello\nworld\n"),
		},
		{
			// '-' and a named file.
			Name:  "stdin_dash_and_file",
			Args:  []string{"-", fileNums},
			Stdin: []byte("a\nb\n"),
		},
		{
			// Named file and '-'.
			Name:  "file_and_stdin_dash",
			Args:  []string{fileA, "-"},
			Stdin: []byte("1\n2\n"),
		},

		// --- R1.4: multiple '-' operands consuming round-robin (AC5) ---
		{
			Name:  "double_dash_round_robin",
			Args:  []string{"-", "-"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
		{
			Name:  "triple_dash_round_robin",
			Args:  []string{"-", "-", "-"},
			Stdin: []byte("1\n2\n3\n4\n5\n6\n"),
		},
		{
			// Multiple dash with file in between.
			Name:  "dash_file_dash",
			Args:  []string{"-", fileA, "-"},
			Stdin: []byte("x\ny\nz\nw\n"),
		},

		// --- R1.3: unequal with stdin ---
		{
			Name:  "stdin_unequal_stdin_shorter",
			Args:  []string{"-", fileLong},
			Stdin: []byte("x\n"),
		},
		{
			Name:  "stdin_unequal_file_shorter",
			Args:  []string{fileShort, "-"},
			Stdin: []byte("a\nb\nc\n"),
		},

		// --- Edge cases ---
		{
			Name:  "empty_stdin",
			Args:  []string{"-"},
			Stdin: []byte{},
		},
		{
			Name: "two_empty_files",
			Args: []string{emptyFile, emptyFile},
		},
		{
			Name: "single_line_file",
			Args: []string{singleLine, fileA},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
