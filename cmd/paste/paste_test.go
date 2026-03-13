// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/paste against gpaste reference binary.
// Implements: prd027-paste R1.1, R1.2, R1.3, R1.4
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
	refBin, err := exec.LookPath("gpaste")
	if err != nil {
		t.Skipf("reference binary gpaste not in PATH: %v", err)
	}

	// Create temp files for test cases.
	dir := t.TempDir()

	// file1: a\nb\n
	file1 := filepath.Join(dir, "file1.txt")
	if err := os.WriteFile(file1, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// file2: 1\n2\n
	file2 := filepath.Join(dir, "file2.txt")
	if err := os.WriteFile(file2, []byte("1\n2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// file3: x\ny\nz\n (longer than file1/file2)
	file3 := filepath.Join(dir, "file3.txt")
	if err := os.WriteFile(file3, []byte("x\ny\nz\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// file4: single line
	file4 := filepath.Join(dir, "file4.txt")
	if err := os.WriteFile(file4, []byte("only\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// empty file
	emptyFile := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	// serial file: a\nb\nc\n
	serialFile := filepath.Join(dir, "serial.txt")
	if err := os.WriteFile(serialFile, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		// R1.1: Two files with tab delimiter.
		{
			Name: "two files default delimiter",
			Args: []string{file1, file2},
		},
		// R1.2: Unequal length files — shorter contributes empty fields.
		{
			Name: "unequal length files",
			Args: []string{file1, file3},
		},
		// R1.2: Three files with different lengths.
		{
			Name: "three files different lengths",
			Args: []string{file4, file1, file3},
		},
		// R1.3: stdin via "-" designator.
		{
			Name: "stdin as dash",
			Args:  []string{"-", file2},
			Stdin: []byte("from_stdin\nline2\n"),
		},
		// R1.4: No files reads from stdin (passthrough).
		{
			Name:  "no files reads stdin",
			Args:  []string{},
			Stdin: []byte("hello\nworld\n"),
		},
		// R1.4: Single dash reads stdin.
		{
			Name:  "single dash reads stdin",
			Args:  []string{"-"},
			Stdin: []byte("one\ntwo\n"),
		},
		// R1.2: File with empty file.
		{
			Name: "file with empty file",
			Args: []string{file1, emptyFile},
		},
		// R1.2: Empty file with file.
		{
			Name: "empty file with file",
			Args: []string{emptyFile, file1},
		},
		// R1.3: Multiple dash arguments each consume next stdin line.
		{
			Name:  "multiple dash args",
			Args:  []string{"-", "-"},
			Stdin: []byte("line1\nline2\nline3\nline4\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
