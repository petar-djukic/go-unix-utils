// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cut against the GNU reference binary (gcut).
// Implements prd026-cut R1.1-R1.4 test coverage: byte selection, character
// selection, range list parsing, and mode exclusivity error handling.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgNameNormalizer replaces the reference binary name (gcut) with
// the Go binary name (cut) in stderr so error message comparisons match.
// Also normalizes the "Try '...gcut --help'" path to "Try 'cut --help'".
func stderrProgNameNormalizer(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("gcut:"), []byte("cut:"))
	// Normalize "Try '/path/to/gcut --help'" to "Try 'cut --help'".
	// The reference binary may include a full path.
	if idx := bytes.Index(data, []byte("Try '")); idx >= 0 {
		if end := bytes.Index(data[idx:], []byte("--help'")); end >= 0 {
			prefix := data[:idx]
			suffix := data[idx+end:]
			data = append(append(prefix, []byte("Try 'cut ")...), suffix...)
		}
	}
	return data
}

// stderrCaseNormalizer lowercases stderr so platform-specific error message
// casing differences do not cause false divergence.
func stderrCaseNormalizer(data []byte) []byte {
	return bytes.ToLower(data)
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcut")
	if err != nil {
		t.Skipf("reference binary gcut not in PATH: %v", err)
	}

	// Create temp files for file-input tests.
	tmpDir := t.TempDir()
	tabFile := filepath.Join(tmpDir, "tab.txt")
	if err := os.WriteFile(tabFile, []byte("a\tb\tc\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	multiLineFile := filepath.Join(tmpDir, "multi.txt")
	if err := os.WriteFile(multiLineFile, []byte("abcdef\nghijkl\nmnopqr\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	colonFile := filepath.Join(tmpDir, "colon.txt")
	if err := os.WriteFile(colonFile, []byte("a:b:c\nd:e:f\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte{}, 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	nonexistentFile := filepath.Join(tmpDir, "nonexistent.txt")

	tests := []testutils.DiffTest{
		// --- R1.1: -b byte selection ---
		{
			Name:  "b_single_byte",
			Args:  []string{"-b", "2"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "b_range",
			Args:  []string{"-b", "2-4"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "b_open_end",
			Args:  []string{"-b", "3-"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "b_open_start",
			Args:  []string{"-b", "-3"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "b_multiple_ranges",
			Args:  []string{"-b", "1,3,5"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "b_overlapping_ranges",
			Args:  []string{"-b", "1-3,2-5"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "b_mixed_range_types",
			Args:  []string{"-b", "1,3-5,7-"},
			Stdin: []byte("abcdefghij\n"),
		},
		{
			// R1.4: line shorter than selected range — only existing bytes output.
			Name:  "b_short_line",
			Args:  []string{"-b", "5-10"},
			Stdin: []byte("abc\n"),
		},
		{
			// R1.4: range entirely beyond line length — empty output line.
			Name:  "b_beyond_line",
			Args:  []string{"-b", "100"},
			Stdin: []byte("abc\n"),
		},
		{
			Name:  "b_multiline",
			Args:  []string{"-b", "2-4"},
			Stdin: []byte("abcdef\nghijkl\n"),
		},

		// --- R1.2: -c character selection (same as -b under LC_ALL=C) ---
		{
			Name:  "c_single_char",
			Args:  []string{"-c", "2"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "c_range",
			Args:  []string{"-c", "2-4"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "c_open_end",
			Args:  []string{"-c", "4-"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "c_open_start",
			Args:  []string{"-c", "-2"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "c_multiple",
			Args:  []string{"-c", "1,3,5"},
			Stdin: []byte("abcdef\n"),
		},

		// --- R1.1: file input ---
		{
			Name: "b_file_input",
			Args: []string{"-b", "2-4", multiLineFile},
		},
		{
			// Read from stdin when no files given.
			Name:  "b_stdin_no_args",
			Args:  []string{"-b", "1-3"},
			Stdin: []byte("hello\n"),
		},
		{
			// '-' means stdin.
			Name:  "b_dash_stdin",
			Args:  []string{"-b", "1-3", "-"},
			Stdin: []byte("hello\n"),
		},
		{
			// Multiple files processed in order.
			Name: "b_multiple_files",
			Args: []string{"-b", "1-3", multiLineFile, colonFile},
		},
		{
			// File and stdin interleaved.
			Name:  "b_file_and_stdin",
			Args:  []string{"-b", "1-3", multiLineFile, "-"},
			Stdin: []byte("stdin_line\n"),
		},

		// --- R1.3: newline handling ---
		{
			// Empty input produces no output.
			Name:  "b_empty_input",
			Args:  []string{"-b", "1"},
			Stdin: []byte{},
		},
		{
			// Empty line (just newline).
			Name:  "b_empty_line",
			Args:  []string{"-b", "1"},
			Stdin: []byte("\n"),
		},
		{
			// No trailing newline.
			Name:  "b_no_trailing_newline",
			Args:  []string{"-b", "1-3"},
			Stdin: []byte("abcdef"),
		},

		// --- R1.4: exactly one of -b/-c/-f required ---
		{
			// No mode specified.
			Name:      "error_no_mode",
			Stdin:     []byte("abc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},

		// --- Range edge cases ---
		{
			// Entire line selected via -b 1-.
			Name:  "b_entire_line",
			Args:  []string{"-b", "1-"},
			Stdin: []byte("abcdef\n"),
		},
		{
			// Adjacent ranges that merge.
			Name:  "b_adjacent_ranges",
			Args:  []string{"-b", "1-3,4-6"},
			Stdin: []byte("abcdef\n"),
		},
		{
			// Duplicate positions.
			Name:  "b_duplicate_positions",
			Args:  []string{"-b", "2,2,2"},
			Stdin: []byte("abcdef\n"),
		},
		{
			// Reversed comma order (not decreasing range, just unordered list).
			Name:  "b_unordered_list",
			Args:  []string{"-b", "5,3,1"},
			Stdin: []byte("abcdef\n"),
		},
		{
			// Short form -b attached.
			Name:  "b_short_form_attached",
			Args:  []string{"-b2-4"},
			Stdin: []byte("abcdef\n"),
		},
		{
			// Short form -c attached.
			Name:  "c_short_form_attached",
			Args:  []string{"-c1,3"},
			Stdin: []byte("abcdef\n"),
		},

		// --- Nonexistent file ---
		{
			Name:      "error_nonexistent_file",
			Args:      []string{"-b", "1", nonexistentFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer, stderrCaseNormalizer},
		},
		{
			// Nonexistent file among valid files — processing continues.
			Name:      "error_nonexistent_with_valid",
			Args:      []string{"-b", "1-3", multiLineFile, nonexistentFile, colonFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer, stderrCaseNormalizer},
		},

		// --- Empty file ---
		{
			Name: "b_empty_file",
			Args: []string{"-b", "1", emptyFile},
		},

		// --- Field mode basics (for R1.4 mode validation) ---
		{
			Name:  "f_basic_field",
			Args:  []string{"-d:", "-f", "2"},
			Stdin: []byte("a:b:c\n"),
		},
		{
			Name:  "f_multiple_fields",
			Args:  []string{"-d:", "-f", "1,3"},
			Stdin: []byte("a:b:c\n"),
		},
		{
			// -s suppress lines without delimiter.
			Name:  "f_suppress_no_delim",
			Args:  []string{"-d:", "-f", "2", "-s"},
			Stdin: []byte("no-delimiter\n"),
		},
		{
			// Line without delimiter and no -s prints unchanged.
			Name:  "f_no_delim_no_suppress",
			Args:  []string{"-d:", "-f", "2"},
			Stdin: []byte("no-delimiter\n"),
		},
		{
			// --complement with -f.
			Name:  "f_complement",
			Args:  []string{"-d:", "-f", "2", "--complement"},
			Stdin: []byte("a:b:c\n"),
		},
		{
			// --output-delimiter with -f.
			Name:  "f_output_delimiter",
			Args:  []string{"-d:", "-f", "1,3", "--output-delimiter=|"},
			Stdin: []byte("a:b:c\n"),
		},
		{
			// --complement with -b.
			Name:  "b_complement",
			Args:  []string{"-b", "2-4", "--complement"},
			Stdin: []byte("abcdef\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
