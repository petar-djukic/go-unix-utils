// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd070-fmt R1.1–R1.4: basic paragraph filling
// and argument parsing.
package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfmt")
	if err != nil {
		t.Skipf("reference binary gfmt not in PATH: %v", err)
	}

	tmpDir := t.TempDir()
	singleLine := filepath.Join(tmpDir, "single.txt")
	writeTestFile(t, singleLine, "short line\n")

	longPara := filepath.Join(tmpDir, "long.txt")
	writeTestFile(t, longPara, "This is a long paragraph that should be reformatted to fit within the default width of seventy-five characters when processed by fmt.\n")

	twoParagraphs := filepath.Join(tmpDir, "two_para.txt")
	writeTestFile(t, twoParagraphs, "First paragraph line one. First paragraph line two.\n\nSecond paragraph line one. Second paragraph line two.\n")

	indentedFile := filepath.Join(tmpDir, "indented.txt")
	writeTestFile(t, indentedFile, "    This is an indented paragraph that has a first line with four spaces of indentation and should be reformatted preserving that indentation.\n")

	multiIndent := filepath.Join(tmpDir, "multi_indent.txt")
	writeTestFile(t, multiIndent, "  First line with two spaces.\n    Second line with four spaces and enough text that it wraps around.\n    Third line also with four spaces.\n")

	emptyFile := filepath.Join(tmpDir, "empty.txt")
	writeTestFile(t, emptyFile, "")

	fileA := filepath.Join(tmpDir, "a.txt")
	writeTestFile(t, fileA, "File A content here.\n")
	fileB := filepath.Join(tmpDir, "b.txt")
	writeTestFile(t, fileB, "File B content here.\n")

	blankLines := filepath.Join(tmpDir, "blanks.txt")
	writeTestFile(t, blankLines, "para one\n\n\npara two\n")

	tests := []testutils.DiffTest{
		// R1.1: short line unchanged at default width.
		{
			Name:  "short_line_unchanged",
			Stdin: []byte("short line\n"),
		},
		// R1.1: long paragraph reformatted to 75 chars.
		{
			Name:  "long_paragraph_wrap",
			Stdin: []byte("This is a long paragraph that should be reformatted to fit within the default width of seventy-five characters when processed by the fmt utility.\n"),
		},
		// R1.2: blank line separates paragraphs.
		{
			Name:  "blank_line_paragraph_separator",
			Stdin: []byte("First paragraph.\n\nSecond paragraph.\n"),
		},
		// R1.2: multiple blank lines preserved as separators.
		{
			Name:  "multiple_blank_lines",
			Stdin: []byte("para one\n\n\npara two\n"),
		},
		// R1.3: indentation preserved on first line.
		{
			Name:  "preserve_first_line_indent",
			Stdin: []byte("    Indented paragraph that has a first line with leading spaces and should reformat preserving that indentation across the wrap.\n"),
		},
		// R1.3: subsequent lines use second line's indentation.
		{
			Name:  "subsequent_line_indent",
			Stdin: []byte("  First line with two spaces.\n    Second line with four spaces and enough text to cause wrapping in the output.\n    Third line also with four spaces.\n"),
		},
		// R1.4: stdin with no arguments.
		{
			Name:  "stdin_no_args",
			Stdin: []byte("hello world\n"),
		},
		// R1.4: stdin via explicit "-".
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("hello world\n"),
		},
		// R1.4: single file argument.
		{
			Name: "single_file",
			Args: []string{singleLine},
		},
		// R1.4: long paragraph from file.
		{
			Name: "file_long_paragraph",
			Args: []string{longPara},
		},
		// R1.4: two paragraphs from file.
		{
			Name: "file_two_paragraphs",
			Args: []string{twoParagraphs},
		},
		// R1.4: multiple file arguments.
		{
			Name: "multiple_files",
			Args: []string{fileA, fileB},
		},
		// R1.4: empty file.
		{
			Name: "empty_file",
			Args: []string{emptyFile},
		},
		// R1.3: indented file.
		{
			Name: "file_indented",
			Args: []string{indentedFile},
		},
		// R1.3: multi-indent file.
		{
			Name: "file_multi_indent",
			Args: []string{multiIndent},
		},
		// R1.2: blank lines from file.
		{
			Name: "file_blank_lines",
			Args: []string{blankLines},
		},
		// R1.4: missing file produces error and exit 1.
		{
			Name:      "missing_file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R1.1: empty stdin.
		{
			Name:  "empty_stdin",
			Stdin: []byte(""),
		},
		// R1.1: single word.
		{
			Name:  "single_word",
			Stdin: []byte("word\n"),
		},
		// R1.1: exactly at width boundary.
		{
			Name:  "at_width_boundary",
			Stdin: []byte("aaaaaaaaa bbbbbbbbb ccccccccc ddddddddd eeeeeeeee fffffffff ggggggggg hh\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// normalizeProgramName replaces the reference binary name (gfmt) with fmt
// in stderr output so that error messages can be compared.
func normalizeProgramName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gfmt:"), []byte("fmt:"))
}

// writeTestFile writes content to a file, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", path, err)
	}
}
