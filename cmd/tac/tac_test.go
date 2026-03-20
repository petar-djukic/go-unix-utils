// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd021-tac R1.1–R1.4: core reversal behavior.
// Differential tests for prd021-tac R2.1–R2.4: separator options (-s, -b, -r).
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
	// R2.1: file with custom separator
	colonFile := createTempFile(t, "a:b:c:")
	// R2.2: file with separator-before pattern
	beforeFile := createTempFile(t, ":a:b:c")

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
			Name:  "stdin_reversal",
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R1.3: stdin with "-" argument
			Name:  "stdin_dash_arg",
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
		{
			// R2.1: -s with colon separator via stdin
			Name:  "custom_sep_colon_stdin",
			Args:  []string{"-s", ":"},
			Stdin: []byte("a:b:c:"),
		},
		{
			// R2.1: -s with colon separator via file
			Name: "custom_sep_colon_file",
			Args: []string{"-s", ":", colonFile},
		},
		{
			// R2.1: -s with no trailing separator
			Name:  "custom_sep_no_trailing",
			Args:  []string{"-s", ":"},
			Stdin: []byte("a:b:c"),
		},
		{
			// R2.1: -s with multi-char separator
			Name:  "custom_sep_multichar",
			Args:  []string{"-s", "::"},
			Stdin: []byte("a::b::c::"),
		},
		{
			// R2.2: -b with default newline separator
			Name:  "before_newline",
			Args:  []string{"-b"},
			Stdin: []byte("\na\nb\nc"),
		},
		{
			// R2.2: -b with custom separator via stdin
			Name:  "before_custom_sep_stdin",
			Args:  []string{"-b", "-s", ":"},
			Stdin: []byte(":a:b:c"),
		},
		{
			// R2.2: -b with custom separator via file
			Name: "before_custom_sep_file",
			Args: []string{"-b", "-s", ":", beforeFile},
		},
		{
			// R2.3: -r with simple regex separator
			Name:  "regex_sep_digits",
			Args:  []string{"-r", "-s", "[0-9]+"},
			Stdin: []byte("a1b22c"),
		},
		{
			// R2.4: -r and -s combined with quantifier
			Name:  "regex_combined_quantifier",
			Args:  []string{"-r", "-s", ":+"},
			Stdin: []byte("a:b::c:::"),
		},
		{
			// R2.3, R2.4: combined short flags -rs
			Name:  "combined_flags_rs",
			Args:  []string{"-rs", ":+"},
			Stdin: []byte("a:b::c"),
		},
		{
			// R2.2, R2.3: -b with regex separator
			Name:  "before_regex",
			Args:  []string{"-b", "-r", "-s", "[0-9]+"},
			Stdin: []byte("1a2b3c"),
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
