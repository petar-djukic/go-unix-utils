// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd023-fold R4.1, R4.2, R4.3, R4.4.
// R4.1: default 80-column folding with various line lengths.
// R4.2: -w flag with multiple widths including edge cases.
// R4.3: -b (byte counting) and -s (space breaking) and their combination.
// R4.4: error cases (nonexistent files, invalid width, unrecognized flags).
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeBinaryName replaces "gfold:" or "fold:" with "PROG:" in stderr
// so that program name differences do not cause false failures.
func normalizeBinaryName(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gfold:"), []byte("PROG:"))
	b = bytes.ReplaceAll(b, []byte("fold:"), []byte("PROG:"))
	return b
}

// normalizeOpenError normalizes Go-style "open <path>: <msg>" to just "<msg>"
// to match GNU-style error messages that omit the syscall name.
var openErrorRe = regexp.MustCompile(`open ([^:]+): `)

func normalizeOpenError(b []byte) []byte {
	return openErrorRe.ReplaceAll(b, []byte("$1: "))
}

// normalizeErrorCase normalizes case differences in error messages
// (e.g., "No such" vs "no such").
func normalizeErrorCase(b []byte) []byte {
	return bytes.ToLower(b)
}

// normalizeWidthSuffix strips trailing system error detail after the quoted
// width value (e.g., ": Result too large") from invalid width messages.
var widthSuffixRe = regexp.MustCompile(`(invalid number of columns: '[^']*'):[^\n]*`)

func normalizeWidthSuffix(b []byte) []byte {
	return widthSuffixRe.ReplaceAll(b, []byte("$1"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfold")
	if err != nil {
		t.Skip("reference binary gfold not in PATH")
	}

	errNorm := []testutils.NormalizeFunc{
		normalizeBinaryName,
		normalizeOpenError,
		normalizeErrorCase,
		normalizeWidthSuffix,
	}

	tests := []testutils.DiffTest{
		// --- R4.1: default 80-column folding ---

		// Empty input produces no output.
		{
			Name:  "empty_input",
			Stdin: []byte(""),
		},

		// Short line passes through unchanged.
		{
			Name:  "short_line_unchanged",
			Stdin: []byte("hello\n"),
		},

		// Line exactly 80 characters passes through unchanged.
		{
			Name:  "exactly_80_chars",
			Stdin: []byte(strings.Repeat("a", 80) + "\n"),
		},

		// Line of 81 chars wraps after column 80.
		{
			Name:  "81_chars_wraps",
			Stdin: []byte(strings.Repeat("b", 81) + "\n"),
		},

		// Line of 160 chars wraps twice.
		{
			Name:  "160_chars_double_wrap",
			Stdin: []byte(strings.Repeat("c", 160) + "\n"),
		},

		// Multiple short lines pass through unchanged.
		{
			Name:  "multiple_short_lines",
			Stdin: []byte("one\ntwo\nthree\n"),
		},

		// Input without trailing newline.
		{
			Name:  "no_trailing_newline",
			Stdin: []byte("no newline at end"),
		},

		// Stdin read when no file arguments given.
		{
			Name:  "stdin_no_args",
			Stdin: []byte("stdin line\n"),
		},

		// Stdin via explicit "-" argument.
		{
			Name:  "stdin_dash_arg",
			Args:  []string{"-"},
			Stdin: []byte("dash stdin\n"),
		},

		// Tab character handling in column mode.
		{
			Name:  "tab_column_counting",
			Stdin: []byte("\thello\n"),
		},

		// --- R4.2: -w flag with multiple widths ---

		// -w 4: wrap at 4 columns.
		{
			Name:  "width_4",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefghij"),
		},

		// -w 1: each character on its own line.
		{
			Name:  "width_1",
			Args:  []string{"-w", "1"},
			Stdin: []byte("abc\n"),
		},

		// -w 1000: very large width, no wrapping needed.
		{
			Name:  "width_1000_no_wrap",
			Args:  []string{"-w", "1000"},
			Stdin: []byte(strings.Repeat("x", 200) + "\n"),
		},

		// -w 10: wrap at 10.
		{
			Name:  "width_10",
			Args:  []string{"-w", "10"},
			Stdin: []byte("1234567890abcde\n"),
		},

		// --width=5 long form.
		{
			Name:  "long_form_width_5",
			Args:  []string{"--width=5"},
			Stdin: []byte("abcdefghij\n"),
		},

		// -w with tab characters.
		{
			Name:  "width_16_with_tabs",
			Args:  []string{"-w", "16"},
			Stdin: []byte("\t\txy\n"),
		},

		// --- R4.3: -b and -s flags and combination ---

		// -b: byte mode, tabs count as 1 byte.
		{
			Name:  "byte_mode_basic",
			Args:  []string{"-b", "-w", "5"},
			Stdin: []byte("abcdefghij\n"),
		},

		// -b: tabs count as 1 byte, not expanded.
		{
			Name:  "byte_mode_tabs",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("\tab\n"),
		},

		// -s: break at word boundaries.
		{
			Name:  "space_break_basic",
			Args:  []string{"-s", "-w", "11"},
			Stdin: []byte("hello world foo bar\n"),
		},

		// -s: no space within width falls back to hard break.
		{
			Name:  "space_break_no_space",
			Args:  []string{"-s", "-w", "5"},
			Stdin: []byte("abcdefghij\n"),
		},

		// -s: space at exact boundary.
		{
			Name:  "space_break_at_boundary",
			Args:  []string{"-s", "-w", "5"},
			Stdin: []byte("abcd efgh\n"),
		},

		// -s: multiple words fitting exactly.
		{
			Name:  "space_break_multiple_words",
			Args:  []string{"-s", "-w", "10"},
			Stdin: []byte("the quick brown fox jumps over the lazy dog\n"),
		},

		// -b -s: combined byte mode and space breaking.
		{
			Name:  "byte_space_combined",
			Args:  []string{"-b", "-s", "-w", "10"},
			Stdin: []byte("hello world foo bar baz\n"),
		},

		// -b -s: with tabs (tabs count as 1 byte in -b mode).
		{
			Name:  "byte_space_with_tabs",
			Args:  []string{"-b", "-s", "-w", "8"},
			Stdin: []byte("ab\tcd efgh\n"),
		},

		// -s -w 1: edge case, width 1 with space breaking.
		{
			Name:  "space_break_width_1",
			Args:  []string{"-s", "-w", "1"},
			Stdin: []byte("a b c\n"),
		},

		// --- R4.4: error cases ---

		// Invalid width: zero.
		{
			Name:      "error_width_zero",
			Args:      []string{"-w", "0"},
			Normalize: errNorm,
		},

		// Invalid width: negative.
		{
			Name:      "error_width_negative",
			Args:      []string{"-w", "-5"},
			Normalize: errNorm,
		},

		// Invalid width: non-numeric.
		{
			Name:      "error_width_non_numeric",
			Args:      []string{"-w", "abc"},
			Normalize: errNorm,
		},

		// Nonexistent file.
		{
			Name:      "error_nonexistent_file",
			Args:      []string{"/nonexistent/file/path"},
			Normalize: errNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffMultiFile tests fold with multiple file arguments.
func TestDiffMultiFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfold")
	if err != nil {
		t.Skip("reference binary gfold not in PATH")
	}

	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "f1.txt")
	file2 := filepath.Join(tmpDir, "f2.txt")
	writeTestFile(t, file1, strings.Repeat("a", 90)+"\n")
	writeTestFile(t, file2, "short line\n")

	tests := []testutils.DiffTest{
		// Multiple files processed in order.
		{
			Name: "multi_file",
			Args: []string{file1, file2},
		},
		// Multiple files with -w.
		{
			Name: "multi_file_width_10",
			Args: []string{"-w", "10", file1, file2},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile writes content to a file, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
