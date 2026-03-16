// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cut against the GNU reference binary (gcut).
// Implements prd026-cut R1.1-R1.4, R2.1-R2.4, R3.1-R3.3, and R4.1-R4.4 test
// coverage: byte selection, character selection, field selection with delimiter
// support, range list parsing, mode exclusivity error handling, stdin input,
// multi-file sequential processing, dash-as-stdin, --version/--help output,
// error messages for invalid options and ranges, and edge cases.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgNameNormalizer replaces the reference binary name (gcut or its
// full path) with the Go binary name (cut) in stderr so error message
// comparisons match. The reference binary may appear as "gcut:", as a full
// path like "/opt/homebrew/bin/gcut:", or in "Try '...gcut --help'" blocks.
func stderrProgNameNormalizer(data []byte) []byte {
	// Replace full-path occurrences first (e.g., "/opt/homebrew/bin/gcut:" → "cut:").
	for {
		idx := bytes.Index(data, []byte("/"))
		if idx < 0 {
			break
		}
		end := bytes.Index(data[idx:], []byte("gcut:"))
		if end < 0 {
			break
		}
		data = append(data[:idx], append([]byte("cut:"), data[idx+end+len("gcut:"):]...)...)
	}
	// Replace bare gcut: occurrences.
	data = bytes.ReplaceAll(data, []byte("gcut:"), []byte("cut:"))
	// Normalize "Try '/path/to/gcut --help'" to "Try 'cut --help'".
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

		// --- R2.1: field selection with ranges ---
		{
			// R2.1: field range N-M.
			Name:  "f_field_range",
			Args:  []string{"-d:", "-f", "2-3"},
			Stdin: []byte("a:b:c:d:e\n"),
		},
		{
			// R2.1: field open-end range N-.
			Name:  "f_field_open_end",
			Args:  []string{"-d:", "-f", "3-"},
			Stdin: []byte("a:b:c:d:e\n"),
		},
		{
			// R2.1: field open-start range -M.
			Name:  "f_field_open_start",
			Args:  []string{"-d:", "-f", "-2"},
			Stdin: []byte("a:b:c:d:e\n"),
		},
		{
			// R2.1: field range beyond available fields — out-of-range produces nothing.
			Name:  "f_field_beyond_range",
			Args:  []string{"-d:", "-f", "10"},
			Stdin: []byte("a:b:c\n"),
		},
		{
			// R2.1: comma-separated combination of ranges.
			Name:  "f_field_combo_ranges",
			Args:  []string{"-d:", "-f", "1,3-4"},
			Stdin: []byte("a:b:c:d:e\n"),
		},

		// --- R2.2: default TAB delimiter ---
		{
			// R2.2: default tab delimiter when -d is not specified.
			Name:  "f_default_tab_delimiter",
			Args:  []string{"-f", "2"},
			Stdin: []byte("a\tb\tc\n"),
		},
		{
			// R2.2: default tab delimiter with multiple fields.
			Name:  "f_default_tab_multiple",
			Args:  []string{"-f", "1,3"},
			Stdin: []byte("a\tb\tc\n"),
		},
		{
			// R2.2: file input with default tab delimiter.
			Name: "f_tab_file_input",
			Args: []string{"-f", "2", tabFile},
		},
		{
			// R2.2: space delimiter.
			Name:  "f_space_delimiter",
			Args:  []string{"-d", " ", "-f", "2"},
			Stdin: []byte("hello world test\n"),
		},

		// --- R2.3: suppress lines without delimiter ---
		{
			// R2.3: -s with mix of lines with and without delimiter.
			Name:  "f_suppress_mixed_lines",
			Args:  []string{"-d:", "-f", "2", "-s"},
			Stdin: []byte("a:b:c\nno-delim\nx:y:z\n"),
		},
		{
			// R2.3: without -s, lines without delimiter pass through unchanged.
			Name:  "f_no_suppress_mixed_lines",
			Args:  []string{"-d:", "-f", "2"},
			Stdin: []byte("a:b:c\nno-delim\nx:y:z\n"),
		},
		{
			// R2.3: --only-delimited long form.
			Name:  "f_only_delimited_long",
			Args:  []string{"-d:", "-f", "1", "--only-delimited"},
			Stdin: []byte("has:delim\nnope\n"),
		},

		// --- R2.4: output delimiter ---
		{
			// R2.4: --output-delimiter with multi-char string.
			Name:  "f_output_delimiter_multichar",
			Args:  []string{"-d:", "-f", "1,2,3", "--output-delimiter=, "},
			Stdin: []byte("a:b:c\n"),
		},
		{
			// R2.4: --output-delimiter with range.
			Name:  "f_output_delimiter_range",
			Args:  []string{"-d:", "-f", "1-3", "--output-delimiter=-"},
			Stdin: []byte("a:b:c:d:e\n"),
		},
		{
			// R2.4: --output-delimiter default is input delimiter.
			Name:  "f_output_delimiter_default_is_input",
			Args:  []string{"-d,", "-f", "1,3"},
			Stdin: []byte("a,b,c\n"),
		},
		{
			// R2.4: --output-delimiter with complement.
			Name:  "f_output_delimiter_complement",
			Args:  []string{"-d:", "-f", "2", "--complement", "--output-delimiter=|"},
			Stdin: []byte("a:b:c:d\n"),
		},

		// --- Field mode edge cases ---
		{
			// Consecutive delimiters produce empty fields.
			Name:  "f_consecutive_delimiters",
			Args:  []string{"-d:", "-f", "1,2,3"},
			Stdin: []byte("a::c\n"),
		},
		{
			// Single field (no delimiter in line with -f1).
			Name:  "f_single_field_no_delim",
			Args:  []string{"-d:", "-f", "1"},
			Stdin: []byte("no-delimiter\n"),
		},
		{
			// -f with multiline input.
			Name:  "f_multiline",
			Args:  []string{"-d:", "-f", "2"},
			Stdin: []byte("a:b:c\nd:e:f\ng:h:i\n"),
		},
		{
			// -f with file input (colon-delimited).
			Name: "f_file_input_colon",
			Args: []string{"-d:", "-f", "1,3", colonFile},
		},
		{
			// -f with short form attached.
			Name:  "f_short_form_attached",
			Args:  []string{"-d:", "-f2"},
			Stdin: []byte("a:b:c\n"),
		},
		{
			// -f with no trailing newline.
			Name:  "f_no_trailing_newline",
			Args:  []string{"-d:", "-f", "2"},
			Stdin: []byte("a:b:c"),
		},
		{
			// -f complement with range.
			Name:  "f_complement_range",
			Args:  []string{"-d:", "-f", "2-3", "--complement"},
			Stdin: []byte("a:b:c:d:e\n"),
		},
		{
			// Empty input with field mode.
			Name:  "f_empty_input",
			Args:  []string{"-d:", "-f", "1"},
			Stdin: []byte{},
		},

		// --- R3.1: read from stdin when no FILE operands given ---
		{
			// R3.1: field mode reads stdin when no files specified.
			Name:  "f_stdin_no_files",
			Args:  []string{"-d:", "-f", "2"},
			Stdin: []byte("a:b:c\nd:e:f\n"),
		},
		{
			// R3.1: field mode stdin with suppress.
			Name:  "f_stdin_suppress",
			Args:  []string{"-d:", "-f", "1", "-s"},
			Stdin: []byte("a:b\nno-delim\nc:d\n"),
		},

		// --- R3.2: multiple FILE operands processed sequentially ---
		{
			// R3.2: two colon-delimited files, field mode.
			Name: "f_multi_file",
			Args: []string{"-d:", "-f", "2", colonFile, colonFile},
		},
		{
			// R3.2: multi-file with byte mode.
			Name: "b_multi_file_concat",
			Args: []string{"-b", "1-2", multiLineFile, colonFile},
		},
		{
			// R3.2: field mode with output delimiter across files.
			Name: "f_multi_file_output_delim",
			Args: []string{"-d:", "-f", "1,3", "--output-delimiter=|", colonFile, colonFile},
		},

		// --- R3.3: dash as stdin ---
		{
			// R3.3: dash alone reads stdin in field mode.
			Name:  "f_dash_stdin",
			Args:  []string{"-d:", "-f", "2", "-"},
			Stdin: []byte("a:b:c\n"),
		},
		{
			// R3.3: dash before a named file.
			Name:  "f_dash_then_file",
			Args:  []string{"-d:", "-f", "1", "-", colonFile},
			Stdin: []byte("x:y:z\n"),
		},
		{
			// R3.3: named file then dash.
			Name:  "f_file_then_dash",
			Args:  []string{"-d:", "-f", "1", colonFile, "-"},
			Stdin: []byte("x:y:z\n"),
		},
		{
			// R3.3: dash mixed among multiple named files.
			Name:  "b_file_dash_file",
			Args:  []string{"-b", "1-2", multiLineFile, "-", colonFile},
			Stdin: []byte("stdin_data\n"),
		},
		{
			// R3.3: multiple dash occurrences — each reads from current stdin position.
			Name:  "f_double_dash",
			Args:  []string{"-d:", "-f", "1", "-", "-"},
			Stdin: []byte("first:line\nsecond:line\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffHelpVersion tests --help and --version output.
// These are not compared against the reference binary since output text differs.
// Instead we verify the exit code is 0 and output goes to stdout.
// R4.1, R4.2.
func TestDiffHelpVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// R4.1: --version exits 0 and prints version info to stdout.
	t.Run("version_exit_0", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(goBin, "--version")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("--version failed: %v", err)
		}
		if len(out) == 0 {
			t.Fatal("--version produced no output")
		}
		if !bytes.Contains(out, []byte("cut")) {
			t.Fatalf("--version output missing 'cut': %s", out)
		}
	})

	// R4.2: --help exits 0 and prints usage to stdout.
	t.Run("help_exit_0", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(goBin, "--help")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("--help failed: %v", err)
		}
		if len(out) == 0 {
			t.Fatal("--help produced no output")
		}
		if !bytes.Contains(out, []byte("Usage:")) {
			t.Fatalf("--help output missing 'Usage:': %s", out)
		}
	})
}

// TestDiffErrors tests error handling via differential testing.
// R4.3: error messages for invalid options, missing required flags, and invalid ranges.
func TestDiffErrors(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcut")
	if err != nil {
		t.Skipf("reference binary gcut not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.3: no -b/-c/-f specified.
		{
			Name:      "error_no_mode_specified",
			Stdin:     []byte("abc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R4.3: invalid option -x.
		{
			Name:      "error_invalid_option",
			Args:      []string{"-x"},
			Stdin:     []byte("abc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R4.3: invalid range -b 0 (positions are 1-based).
		{
			Name:      "error_range_zero",
			Args:      []string{"-b", "0"},
			Stdin:     []byte("abc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R4.3: invalid range -b abc (non-numeric).
		{
			Name:      "error_range_nonnumeric",
			Args:      []string{"-b", "abc"},
			Stdin:     []byte("abc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R4.3: decreasing range -b 5-2.
		{
			Name:      "error_decreasing_range",
			Args:      []string{"-b", "5-2"},
			Stdin:     []byte("abc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R4.3: only one mode may be specified (both -b and -f).
		{
			Name:      "error_multiple_modes",
			Args:      []string{"-b", "1", "-f", "1"},
			Stdin:     []byte("abc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R4.3: delimiter must be single character.
		{
			Name:      "error_multichar_delimiter",
			Args:      []string{"-d", "ab", "-f", "1"},
			Stdin:     []byte("abc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R4.3: empty range list.
		{
			Name:      "error_empty_list",
			Args:      []string{"-b", ""},
			Stdin:     []byte("abc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffEdgeCases tests R4.4 edge cases via differential testing.
func TestDiffEdgeCases(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcut")
	if err != nil {
		t.Skipf("reference binary gcut not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.4: empty input produces empty output.
		{
			Name:  "edge_empty_input_bytes",
			Args:  []string{"-b", "1"},
			Stdin: []byte{},
		},
		{
			Name:  "edge_empty_input_fields",
			Args:  []string{"-d:", "-f", "1"},
			Stdin: []byte{},
		},
		// R4.4: single-character delimiter works correctly.
		{
			Name:  "edge_single_char_delim_tab",
			Args:  []string{"-f", "2"},
			Stdin: []byte("a\tb\tc\n"),
		},
		{
			Name:  "edge_single_char_delim_pipe",
			Args:  []string{"-d|", "-f", "2"},
			Stdin: []byte("a|b|c\n"),
		},
		{
			Name:  "edge_single_char_delim_space",
			Args:  []string{"-d", " ", "-f", "2"},
			Stdin: []byte("a b c\n"),
		},
		// R4.4: --output-delimiter with multi-byte string.
		{
			Name:  "edge_output_delim_multichar",
			Args:  []string{"-d:", "-f", "1,2,3", "--output-delimiter=, "},
			Stdin: []byte("a:b:c\n"),
		},
		{
			Name:  "edge_output_delim_longer",
			Args:  []string{"-d:", "-f", "1-3", "--output-delimiter= -> "},
			Stdin: []byte("a:b:c\n"),
		},
		// R4.4: --output-delimiter with single-char string.
		{
			Name:  "edge_output_delim_single_char",
			Args:  []string{"-d:", "-f", "1,2,3", "--output-delimiter=|"},
			Stdin: []byte("a:b:c\n"),
		},
		// R4.4: line with only delimiters (empty fields).
		{
			Name:  "edge_only_delimiters",
			Args:  []string{"-d:", "-f", "1,2,3"},
			Stdin: []byte("::\n"),
		},
		// R4.4: line containing just the delimiter.
		{
			Name:  "edge_single_delimiter",
			Args:  []string{"-d:", "-f", "1,2"},
			Stdin: []byte(":\n"),
		},
		// R4.4: single byte line.
		{
			Name:  "edge_single_byte_line",
			Args:  []string{"-b", "1"},
			Stdin: []byte("x\n"),
		},
		// R4.4: --output-delimiter applies to byte/char mode too.
		{
			Name:  "edge_output_delim_byte_mode",
			Args:  []string{"-b", "1,3,5", "--output-delimiter=_"},
			Stdin: []byte("abcdef\n"),
		},
		// R4.4: complement with single-field input.
		{
			Name:  "edge_complement_single_field",
			Args:  []string{"-d:", "-f", "1", "--complement"},
			Stdin: []byte("only\n"),
		},
		// R4.4: complement with fields on delimiter-containing line.
		{
			Name:  "edge_complement_all_fields",
			Args:  []string{"-d:", "-f", "1-", "--complement"},
			Stdin: []byte("a:b:c\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
