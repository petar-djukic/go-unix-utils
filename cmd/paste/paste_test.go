// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/paste: differential testing against gpaste.
// Implements srd027-paste R2.1, R2.2, R2.3, R3.1, R3.2, R3.3.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// writeTestFile creates a file with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file %s: %v", path, err)
	}
}

// TestDiffParallel tests parallel merge mode with delimiter configuration.
// R2.1: -d DELIM sets the separator; delimiter list cycles across fields.
// R2.2: escape sequences \n, \t, \\, \0 are recognized.
// R2.3: cycling resets from the first delimiter for each new output line.
func TestDiffParallel(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpaste")
	if err != nil {
		t.Skipf("reference binary gpaste not in PATH: %v", err)
	}

	dir := t.TempDir()
	file1 := filepath.Join(dir, "f1.txt")
	file2 := filepath.Join(dir, "f2.txt")
	file3 := filepath.Join(dir, "f3.txt")
	writeTestFile(t, file1, "a\nb\nc\n")
	writeTestFile(t, file2, "1\n2\n3\n")
	writeTestFile(t, file3, "x\ny\nz\n")

	tests := []testutils.DiffTest{
		// R2.1: default tab delimiter with two files
		{Name: "two_files_tab", Args: []string{file1, file2}},
		// R2.1: custom single-char delimiter
		{Name: "custom_delim_colon", Args: []string{"-d", ":", file1, file2}},
		// R2.1: custom delimiter with -d inline
		{Name: "custom_delim_inline", Args: []string{"-d:", file1, file2}},
		// R2.1: three files with default tab
		{Name: "three_files_tab", Args: []string{file1, file2, file3}},
		// R2.1: delimiter list cycles across fields (R2.3: resets per line)
		{Name: "delim_list_cycle", Args: []string{"-d", ":,", file1, file2, file3}},
		// R2.2: escape sequence \t (explicit tab)
		{Name: "escape_tab", Args: []string{"-d", `\t`, file1, file2}},
		// R2.2: escape sequence \n (newline as delimiter)
		{Name: "escape_newline", Args: []string{"-d", `\n`, file1, file2}},
		// R2.2: escape sequence \\ (literal backslash)
		{Name: "escape_backslash", Args: []string{"-d", `\\`, file1, file2}},
		// R2.2: escape sequence \0 (empty string, no delimiter)
		{Name: "escape_null", Args: []string{"-d", `\0`, file1, file2}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffUnequalLength tests parallel merge with files of different lengths.
// R2.1: exhausted files contribute empty fields; delimiter still appears.
func TestDiffUnequalLength(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpaste")
	if err != nil {
		t.Skipf("reference binary gpaste not in PATH: %v", err)
	}

	dir := t.TempDir()
	short := filepath.Join(dir, "short.txt")
	long := filepath.Join(dir, "long.txt")
	writeTestFile(t, short, "a\nb\n")
	writeTestFile(t, long, "1\n2\n3\n4\n5\n")

	tests := []testutils.DiffTest{
		// Short file exhausted before long; empty fields appear
		{Name: "short_then_long", Args: []string{short, long}},
		{Name: "long_then_short", Args: []string{long, short}},
		// Custom delimiter with unequal lengths
		{Name: "unequal_custom_delim", Args: []string{"-d", ":", short, long}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffStdin tests stdin handling via '-' operand.
// R2.3/R1.3: stdin is represented by '-' and can appear in the file list.
func TestDiffStdin(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpaste")
	if err != nil {
		t.Skipf("reference binary gpaste not in PATH: %v", err)
	}

	dir := t.TempDir()
	file1 := filepath.Join(dir, "f1.txt")
	writeTestFile(t, file1, "x\ny\nz\n")

	tests := []testutils.DiffTest{
		// stdin as first operand
		{Name: "stdin_first", Args: []string{"-", file1}, Stdin: []byte("a\nb\nc\n")},
		// stdin as second operand
		{Name: "stdin_second", Args: []string{file1, "-"}, Stdin: []byte("1\n2\n3\n")},
		// stdin with custom delimiter
		{Name: "stdin_custom_delim", Args: []string{"-d", ":", "-", file1}, Stdin: []byte("a\nb\nc\n")},
		// stdin only (passthrough)
		{Name: "stdin_only", Args: []string{"-"}, Stdin: []byte("hello\nworld\n")},
		// no args defaults to stdin
		{Name: "no_args_stdin", Stdin: []byte("hello\nworld\n")},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffSerial tests serial mode (-s) where each file produces one output line.
// R3.1: all lines of one file joined with delimiter on a single output line.
// R3.2: delimiter list cycles across fields within the output line.
// R3.3: -s overrides parallel mode.
func TestDiffSerial(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpaste")
	if err != nil {
		t.Skipf("reference binary gpaste not in PATH: %v", err)
	}

	dir := t.TempDir()
	file1 := filepath.Join(dir, "f1.txt")
	file2 := filepath.Join(dir, "f2.txt")
	file3 := filepath.Join(dir, "f3.txt")
	writeTestFile(t, file1, "a\nb\nc\n")
	writeTestFile(t, file2, "1\n2\n3\n")
	writeTestFile(t, file3, "x\ny\n")

	tests := []testutils.DiffTest{
		// R3.1: single file, all lines joined with tab
		{Name: "serial_one_file", Args: []string{"-s", file1}},
		// R3.1: two files, each produces one output line
		{Name: "serial_two_files", Args: []string{"-s", file1, file2}},
		// R3.1: three files with default tab delimiter
		{Name: "serial_three_files", Args: []string{"-s", file1, file2, file3}},
		// R3.2: custom delimiter in serial mode
		{Name: "serial_custom_delim", Args: []string{"-s", "-d", ":", file1, file2}},
		// R3.2: delimiter list cycles in serial mode
		{Name: "serial_delim_cycle", Args: []string{"-s", "-d", ":,", file1}},
		// R3.1: serial mode with stdin via '-'
		{Name: "serial_stdin", Args: []string{"-s", "-"}, Stdin: []byte("a\nb\nc\n")},
		// R3.3: --serial long form
		{Name: "serial_long_flag", Args: []string{"--serial", file1}},
		// R3.1: serial mode with unequal-length files
		{Name: "serial_unequal", Args: []string{"-s", file1, file3}},
		// R3.2: escape sequence delimiter in serial mode
		{Name: "serial_escape_null", Args: []string{"-s", "-d", `\0`, file1}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffDelimLong tests --delimiters= long form.
// R2.1: --delimiters=LIST sets the delimiter list.
func TestDiffDelimLong(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpaste")
	if err != nil {
		t.Skipf("reference binary gpaste not in PATH: %v", err)
	}

	dir := t.TempDir()
	file1 := filepath.Join(dir, "f1.txt")
	file2 := filepath.Join(dir, "f2.txt")
	writeTestFile(t, file1, "a\nb\n")
	writeTestFile(t, file2, "1\n2\n")

	tests := []testutils.DiffTest{
		{Name: "long_delimiters_eq", Args: []string{"--delimiters=:", file1, file2}},
		{Name: "long_delimiters_sep", Args: []string{"--delimiters", "|", file1, file2}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
