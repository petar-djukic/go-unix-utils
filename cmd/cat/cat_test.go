// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd006-cat R1.5, R2.1–R2.4, R3.1–R3.3.
package main_test

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
	refBin, err := exec.LookPath("gcat")
	if err != nil {
		t.Skipf("reference binary gcat not in PATH: %v", err)
	}

	// Create temp files for cross-file boundary tests (R3.2).
	tmpDir := t.TempDir()
	fileEndBlank := filepath.Join(tmpDir, "end_blank.txt")
	fileStartBlank := filepath.Join(tmpDir, "start_blank.txt")
	writeTestFile(t, fileEndBlank, "a\n\n")
	writeTestFile(t, fileStartBlank, "\nb\n")

	tests := []testutils.DiffTest{
		// R1.2, R1.5: stdin with no arguments.
		{
			Name:  "stdin_no_args",
			Stdin: []byte("hello\nworld\n"),
		},
		// R1.2: stdin via explicit "-".
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("hello\nworld\n"),
		},
		// R1.5: no trailing newline preserved.
		{
			Name:  "stdin_no_trailing_newline",
			Stdin: []byte("hello"),
		},
		// R3.1: squeeze multiple consecutive blank lines.
		{
			Name:  "squeeze_multiple_blanks",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		// R3.1: single blank line passes through.
		{
			Name:  "squeeze_single_blank",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\nb\n"),
		},
		// R3.1: no blanks, nothing to squeeze.
		{
			Name:  "squeeze_no_blanks",
			Args:  []string{"-s"},
			Stdin: []byte("a\nb\n"),
		},
		// R3.2: squeeze across file boundaries.
		{
			Name: "squeeze_across_files",
			Args: []string{"-s", fileEndBlank, fileStartBlank},
		},
		// R2.1: number all output lines.
		{
			Name:  "number_all",
			Args:  []string{"-n"},
			Stdin: []byte("a\n\nb\n"),
		},
		// R2.2: number only non-blank lines.
		{
			Name:  "number_nonblank",
			Args:  []string{"-b"},
			Stdin: []byte("a\n\nb\n"),
		},
		// R2.3: -b overrides -n when both given.
		{
			Name:  "b_overrides_n",
			Args:  []string{"-n", "-b"},
			Stdin: []byte("a\n\nb\n"),
		},
		// R3.3: squeeze applied before numbering with -n.
		{
			Name:  "squeeze_with_number",
			Args:  []string{"-sn"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		// R3.3: squeeze applied before numbering with -b.
		{
			Name:  "squeeze_with_number_nonblank",
			Args:  []string{"-sb"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		// R2.4: line with spaces is not blank.
		{
			Name:  "space_line_not_blank",
			Args:  []string{"-b"},
			Stdin: []byte("a\n \nb\n"),
		},
		// R2.4: line with tab is not blank.
		{
			Name:  "tab_line_not_blank",
			Args:  []string{"-b"},
			Stdin: []byte("a\n\tb\n"),
		},
		// R1.5, R2.1: numbering preserves no-trailing-newline.
		{
			Name:  "number_no_trailing_newline",
			Args:  []string{"-n"},
			Stdin: []byte("hello"),
		},
		// R2.1: numbering across multiple lines.
		{
			Name:  "number_many_lines",
			Args:  []string{"-n"},
			Stdin: []byte("a\nb\nc\nd\ne\n"),
		},
		// R3.1: leading blank lines squeezed.
		{
			Name:  "squeeze_leading_blanks",
			Args:  []string{"-s"},
			Stdin: []byte("\n\n\na\n"),
		},
		// R3.1: trailing blank lines squeezed.
		{
			Name:  "squeeze_trailing_blanks",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\n\n\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile writes content to path, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", path, err)
	}
}
