// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cat against gcat (GNU coreutils).
// Implements prd006-cat R1.5, R2.1-R2.3 test coverage.
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
	refBin, err := exec.LookPath("gcat")
	if err != nil {
		t.Skipf("reference binary gcat not in PATH: %v", err)
	}

	// Create test fixtures in a temp directory.
	tmpDir := t.TempDir()
	writeTestFile(t, tmpDir, "hello.txt", "hello\nworld\n")
	writeTestFile(t, tmpDir, "no-trailing-newline.txt", "abc\ndef")
	writeTestFile(t, tmpDir, "blanks.txt", "a\n\n\n\nb\n")
	writeTestFile(t, tmpDir, "single-line.txt", "one\n")
	writeTestFile(t, tmpDir, "empty.txt", "")

	tests := []testutils.DiffTest{
		// R1.5: no newlines added or removed — no trailing newline preserved.
		{
			Name:    "R1.5_no_trailing_newline_preserved",
			Args:    []string{filepath.Join(tmpDir, "no-trailing-newline.txt")},
			WorkDir: tmpDir,
		},
		// R1.5: empty file produces no output.
		{
			Name:    "R1.5_empty_file",
			Args:    []string{filepath.Join(tmpDir, "empty.txt")},
			WorkDir: tmpDir,
		},
		// R1.5: no newlines added via stdin.
		{
			Name:  "R1.5_stdin_no_trailing_newline",
			Stdin: []byte("abc\ndef"),
		},

		// R2.1: -n numbers all lines.
		{
			Name:  "R2.1_number_all_lines",
			Args:  []string{"-n"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R2.1: -n numbers blank lines too.
		{
			Name:  "R2.1_number_blank_lines",
			Args:  []string{"-n"},
			Stdin: []byte("a\n\n\nb\n"),
		},
		// R2.1: -n with no trailing newline.
		{
			Name:  "R2.1_number_no_trailing_newline",
			Args:  []string{"-n"},
			Stdin: []byte("a\nb"),
		},
		// R2.1: -n numbering continues across files.
		{
			Name: "R2.1_number_across_files",
			Args: []string{
				"-n",
				filepath.Join(tmpDir, "single-line.txt"),
				filepath.Join(tmpDir, "hello.txt"),
			},
			WorkDir: tmpDir,
		},
		// R2.1: -n with single line.
		{
			Name:  "R2.1_single_line",
			Args:  []string{"-n"},
			Stdin: []byte("only\n"),
		},
		// R2.1: -n with empty stdin.
		{
			Name:  "R2.1_empty_stdin",
			Args:  []string{"-n"},
			Stdin: []byte(""),
		},

		// R2.2: -b numbers only non-blank lines.
		{
			Name:  "R2.2_number_nonblank",
			Args:  []string{"-b"},
			Stdin: []byte("a\n\n\nb\n"),
		},
		// R2.2: -b blank lines have no prefix.
		{
			Name:  "R2.2_blank_no_prefix",
			Args:  []string{"-b"},
			Stdin: []byte("\n\na\n\nb\n\n"),
		},
		// R2.2: -b with spaces-only line (not blank per R2.4).
		{
			Name:  "R2.2_spaces_not_blank",
			Args:  []string{"-b"},
			Stdin: []byte("a\n  \n\nb\n"),
		},

		// R2.3: -b overrides -n when both given.
		{
			Name:  "R2.3_b_overrides_n",
			Args:  []string{"-n", "-b"},
			Stdin: []byte("a\n\nb\n"),
		},
		// R2.3: -nb combined flag.
		{
			Name:  "R2.3_nb_combined",
			Args:  []string{"-nb"},
			Stdin: []byte("a\n\nb\n"),
		},
		// R2.3: -bn order (b first, n second).
		{
			Name:  "R2.3_bn_combined",
			Args:  []string{"-bn"},
			Stdin: []byte("a\n\nb\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", name, err)
	}
}
