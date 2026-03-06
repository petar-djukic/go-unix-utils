// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/fold (prd023-fold R4).
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests comparing Go fold against gfold.
// R4: byte-for-byte comparison via RunDiffTests.
// D4: LC_ALL=C set via DiffTest.Env.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// D2: Graceful skip if gfold is not in PATH.
	refBin, err := exec.LookPath("gfold")
	if err != nil {
		t.Skipf("reference binary gfold not in PATH: %v", err)
	}

	// Create temp files for file-based tests.
	dir := t.TempDir()
	file1 := writeTestFile(t, dir, "f1.txt", "alpha beta gamma delta epsilon\n")
	file2 := writeTestFile(t, dir, "f2.txt", "short\n")
	fileLong := writeTestFile(t, dir, "long.txt", strings.Repeat("x", 100)+"\n")

	tests := []testutils.DiffTest{
		// R1.2: Lines up to 80 characters pass through unchanged.
		{
			Name:  "fold_default_width_80",
			Stdin: []byte("short line\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1, R1.3: -w 4 wraps a 10-character string into segments.
		{
			Name:  "fold_wrap_at_width",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefghij"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: -s breaks at last space within width.
		{
			Name:  "fold_space_break",
			Args:  []string{"-w", "11", "-s"},
			Stdin: []byte("hello world foo\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: -b measures width in bytes.
		{
			Name:  "fold_byte_mode",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("abcde"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: Tab expands to tab stops for column counting.
		{
			Name:  "fold_tab_columns",
			Args:  []string{"-w", "9"},
			Stdin: []byte("a\tbcde\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: -s with no space falls back to hard wrap.
		{
			Name:  "fold_space_break_no_space",
			Args:  []string{"-w", "4", "-s"},
			Stdin: []byte("abcdefgh\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: Line exactly 80 chars passes through unchanged.
		{
			Name:  "fold_exactly_80",
			Stdin: []byte(strings.Repeat("a", 80) + "\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: Line longer than 80 chars wraps.
		{
			Name:  "fold_over_80",
			Stdin: []byte(strings.Repeat("b", 100) + "\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: No trailing newline on input preserved.
		{
			Name:  "fold_no_trailing_newline",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abcdefghij"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Read from file argument.
		{
			Name: "fold_file_arg",
			Args: []string{"-w", "10", file1},
			Env:  []string{"LC_ALL=C"},
		},
		// Multiple files.
		{
			Name: "fold_multifile",
			Args: []string{"-w", "10", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// Stdin via "-" argument.
		{
			Name:  "fold_stdin_dash",
			Args:  []string{"-w", "5", "-"},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Empty input produces no output.
		{
			Name:  "fold_empty_input",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// File with long line.
		{
			Name: "fold_file_long_line",
			Args: []string{fileLong},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: -s combined with -b.
		{
			Name:  "fold_space_break_byte_mode",
			Args:  []string{"-b", "-s", "-w", "10"},
			Stdin: []byte("hello world foo bar\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Multiple lines with different lengths.
		{
			Name:  "fold_multiple_lines",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abc\nabcdefghij\nxy\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// -w 1 wraps every character.
		{
			Name:  "fold_width_1",
			Args:  []string{"-w", "1"},
			Stdin: []byte("abc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Tab in byte mode counts as 1 byte.
		{
			Name:  "fold_tab_byte_mode",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("a\tbcde\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// -s with spaces throughout.
		{
			Name:  "fold_space_break_multiple_words",
			Args:  []string{"-w", "10", "-s"},
			Stdin: []byte("one two three four five six\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Empty line passes through.
		{
			Name:  "fold_empty_line",
			Stdin: []byte("\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile creates a file in dir with the given content and returns its path.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file %s: %v", name, err)
	}
	return path
}
