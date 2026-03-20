// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd027-paste R1.1–R1.4, R2.1–R2.3.
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

	dir := t.TempDir()
	setupTestFiles(t, dir)

	fa := filepath.Join(dir, "a.txt")
	fb := filepath.Join(dir, "b.txt")
	fc := filepath.Join(dir, "c.txt")
	fshort := filepath.Join(dir, "short.txt")
	fempty := filepath.Join(dir, "empty.txt")

	tests := []testutils.DiffTest{
		// R1 tests
		{
			Name: "two_files_equal_length",
			Args: []string{fa, fb},
		},
		{
			Name: "three_files",
			Args: []string{fa, fb, fc},
		},
		{
			Name: "first_file_shorter",
			Args: []string{fshort, fb},
		},
		{
			Name: "second_file_shorter",
			Args: []string{fa, fshort},
		},
		{
			Name: "single_file",
			Args: []string{fa},
		},
		{
			Name: "empty_and_nonempty",
			Args: []string{fempty, fa},
		},
		{
			Name: "nonempty_and_empty",
			Args: []string{fa, fempty},
		},
		{
			Name: "stdin_passthrough_no_args",
			Stdin: []byte("hello\nworld\n"),
		},
		{
			Name:  "stdin_single_dash",
			Args:  []string{"-"},
			Stdin: []byte("hello\nworld\n"),
		},
		{
			Name:  "stdin_double_dash",
			Args:  []string{"-", "-"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
		{
			Name:  "file_and_stdin",
			Args:  []string{fa, "-"},
			Stdin: []byte("X\nY\n"),
		},
		{
			Name:  "stdin_and_file",
			Args:  []string{"-", fa},
			Stdin: []byte("X\nY\n"),
		},
		// R2.1: single custom delimiter
		{
			Name: "delim_colon_two_files",
			Args: []string{"-d:", fa, fb},
		},
		{
			Name: "delim_comma_three_files",
			Args: []string{"-d,", fa, fb, fc},
		},
		// R2.1: delimiter list cycles across fields
		{
			Name: "delim_list_cycling",
			Args: []string{"-d", ":,", fa, fb, fc},
		},
		// R2.2: escape sequences
		{
			Name: "delim_escape_tab",
			Args: []string{`-d\t`, fa, fb},
		},
		{
			Name: "delim_escape_newline",
			Args: []string{`-d\n`, fa, fb},
		},
		{
			Name: "delim_escape_backslash",
			Args: []string{`-d\\`, fa, fb},
		},
		{
			Name: "delim_escape_empty",
			Args: []string{`-d\0`, fa, fb},
		},
		// R2.2: mixed escapes in delimiter list
		{
			Name: "delim_mixed_escapes",
			Args: []string{`-d`, `:\0`, fa, fb, fc},
		},
		// R2.3: cycling resets per output line
		{
			Name: "delim_cycle_reset_per_line",
			Args: []string{"-d", ":-", fa, fb, fc},
		},
		// R2.1/R2.3: with unequal file lengths
		{
			Name: "delim_custom_unequal_lengths",
			Args: []string{"-d:", fshort, fb},
		},
		{
			Name: "delim_list_unequal_three_files",
			Args: []string{"-d", ":-", fshort, fb, fc},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// setupTestFiles creates the fixture files used by differential tests.
func setupTestFiles(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "a.txt"), "a\nb\n")
	writeFile(t, filepath.Join(dir, "b.txt"), "1\n2\n")
	writeFile(t, filepath.Join(dir, "c.txt"), "x\ny\nz\n")
	writeFile(t, filepath.Join(dir, "short.txt"), "s\n")
	writeFile(t, filepath.Join(dir, "empty.txt"), "")
}

// writeFile writes content to a file, failing the test on error.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}
