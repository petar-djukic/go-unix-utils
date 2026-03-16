// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/expand against the GNU reference binary (gexpand).
// Implements prd024-expand R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.3 test coverage.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgNameNormalizer replaces the reference binary name (gexpand) with
// the Go binary name (expand) in stderr so error message comparisons match.
func stderrProgNameNormalizer(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gexpand:"), []byte("expand:"))
}

// stderrCaseNormalizer lowercases stderr so platform-specific error message
// casing differences (e.g., "No such file" vs "no such file") do not cause
// false divergence.
func stderrCaseNormalizer(data []byte) []byte {
	return bytes.ToLower(data)
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gexpand")
	if err != nil {
		t.Skipf("reference binary gexpand not in PATH: %v", err)
	}

	// Create temp files for file-input tests.
	tmpDir := t.TempDir()
	tabFile := filepath.Join(tmpDir, "tabs.txt")
	if err := os.WriteFile(tabFile, []byte("a\tb\tc\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	noTabFile := filepath.Join(tmpDir, "notabs.txt")
	if err := os.WriteFile(noTabFile, []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// R3.4: files for multi-file concatenation test.
	fileA := filepath.Join(tmpDir, "a.txt")
	if err := os.WriteFile(fileA, []byte("\tline1\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	fileB := filepath.Join(tmpDir, "b.txt")
	if err := os.WriteFile(fileB, []byte("\tline2\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// R3.1: file with leading and embedded tabs for -i tests.
	initialFile := filepath.Join(tmpDir, "initial.txt")
	if err := os.WriteFile(initialFile, []byte("\t\thello\tworld\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// R4.2: empty file for edge case tests.
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte{}, 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// R4.2: file containing only tabs.
	tabOnlyFile := filepath.Join(tmpDir, "tabonly.txt")
	if err := os.WriteFile(tabOnlyFile, []byte("\t\t\t\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// R4.2: file with very long line.
	longLineFile := filepath.Join(tmpDir, "longline.txt")
	longLine := strings.Repeat("abcdefg\t", 100) + "\n"
	if err := os.WriteFile(longLineFile, []byte(longLine), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// R4.2: file with binary content (null bytes).
	binaryFile := filepath.Join(tmpDir, "binary.txt")
	if err := os.WriteFile(binaryFile, []byte("abc\x00\tdef\x00\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// R4.1: nonexistent file path for error handling tests.
	nonexistentFile := filepath.Join(tmpDir, "nonexistent.txt")

	tests := []testutils.DiffTest{
		{
			// R1.1: single tab expanded to 8-column boundary.
			Name:  "single_tab_default",
			Stdin: []byte("a\tb\n"),
		},
		{
			// R1.1: tab at column 1 advances to column 9.
			Name:  "tab_at_start",
			Stdin: []byte("\thello\n"),
		},
		{
			// R1.2: multiple consecutive tabs each advance independently.
			Name:  "consecutive_tabs",
			Stdin: []byte("a\t\tb\n"),
		},
		{
			// R1.2: three consecutive tabs.
			Name:  "three_consecutive_tabs",
			Stdin: []byte("\t\t\tX\n"),
		},
		{
			// R1.3: input with no tabs passes through unchanged.
			Name:  "no_tabs_passthrough",
			Stdin: []byte("hello world\n"),
		},
		{
			// R1.3: each byte counts as one column under LC_ALL=C.
			Name:  "bytes_as_columns",
			Stdin: []byte("1234567\tX\n"),
		},
		{
			// R1.3: tab after 4 chars fills to column 8.
			Name:  "tab_after_four_chars",
			Stdin: []byte("abcd\te\n"),
		},
		{
			// R1.3: tab after exactly 8 chars fills to column 16.
			Name:  "tab_after_eight_chars",
			Stdin: []byte("12345678\tX\n"),
		},
		{
			// R1.4: newline resets column position.
			Name:  "newline_resets_column",
			Stdin: []byte("a\tb\na\tb\n"),
		},
		{
			// R1.4: multiple lines with varying tab positions.
			Name:  "multiline_varying_tabs",
			Stdin: []byte("\tA\nab\tB\nabc\tC\n"),
		},
		{
			// R1.1: read from stdin when no file arguments.
			Name:  "stdin_default",
			Stdin: []byte("x\ty\tz\n"),
		},
		{
			// R1.1: empty input.
			Name:  "empty_input",
			Stdin: []byte{},
		},
		{
			// R1.4: single newline only.
			Name:  "single_newline",
			Stdin: []byte("\n"),
		},
		{
			// R1.3: no trailing newline.
			Name:  "no_trailing_newline",
			Stdin: []byte("a\tb"),
		},
		{
			// R1.2: tabs only.
			Name:  "tabs_only",
			Stdin: []byte("\t\t\t\n"),
		},
		{
			// R1.1: read from named file.
			Name: "read_file",
			Args: []string{tabFile},
		},
		{
			// R1.1: read from file with no tabs.
			Name: "read_file_no_tabs",
			Args: []string{noTabFile},
		},
		{
			// R1.1: read from multiple files in order.
			Name: "multiple_files",
			Args: []string{tabFile, noTabFile},
		},
		{
			// R1.4: '-' means stdin.
			Name:  "dash_stdin",
			Args:  []string{"-"},
			Stdin: []byte("a\tb\n"),
		},
		{
			// R1.4: file and '-' interspersed.
			Name:  "file_and_dash",
			Args:  []string{tabFile, "-"},
			Stdin: []byte("from\tstdin\n"),
		},
		{
			// R1.3: long line with mixed tabs and text.
			Name:  "long_mixed_line",
			Stdin: []byte("a\tbb\tccc\tdddd\teeeee\n"),
		},
		{
			// R1.4: multiple empty lines.
			Name:  "multiple_empty_lines",
			Stdin: []byte("\n\n\n"),
		},
		{
			// R1.2: tab at every position 1-8.
			Name:  "tab_positions_1_through_8",
			Stdin: []byte("1\t\n12\t\n123\t\n1234\t\n12345\t\n123456\t\n1234567\t\n12345678\t\n"),
		},
		{
			// R1.3: multibyte UTF-8 under LC_ALL=C (each byte = 1 column).
			Name:  "multibyte_utf8_lc_c",
			Stdin: []byte("\xc3\xa9\tb\n"),
		},
		{
			// R1.1: input with spaces (not tabs) unchanged.
			Name:  "spaces_unchanged",
			Stdin: []byte("a   b\n"),
		},
		{
			// R1.1: mixed tabs and spaces.
			Name:  "mixed_tabs_spaces",
			Stdin: []byte("a \tb\n"),
		},
		{
			// R1.3: long line of tabs.
			Name:  "many_tabs",
			Stdin: []byte(strings.Repeat("\t", 20) + "X\n"),
		},
		// R2.1: -t N sets uniform tab stop interval.
		{
			Name:  "t_single_value_4",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a\tb\n"),
		},
		{
			Name:  "t_single_value_2",
			Args:  []string{"-t", "2"},
			Stdin: []byte("a\tb\tc\n"),
		},
		{
			Name:  "t_single_value_12",
			Args:  []string{"-t", "12"},
			Stdin: []byte("\tA\n"),
		},
		{
			// R2.1: -t with consecutive tabs using uniform interval.
			Name:  "t_uniform_consecutive_tabs",
			Args:  []string{"-t", "4"},
			Stdin: []byte("\t\t\tX\n"),
		},
		{
			// R2.1: -tN short form.
			Name:  "t_short_form",
			Args:  []string{"-t4"},
			Stdin: []byte("a\tb\n"),
		},
		{
			// R2.1: --tabs=N long form.
			Name:  "tabs_long_form",
			Args:  []string{"--tabs=4"},
			Stdin: []byte("a\tb\n"),
		},
		// R2.2: -t LIST sets explicit tab stop positions.
		{
			Name:  "t_list_three_stops",
			Args:  []string{"-t", "4,8,12"},
			Stdin: []byte("a\tb\tc\td\te\n"),
		},
		{
			Name:  "t_list_two_stops",
			Args:  []string{"-t", "5,10"},
			Stdin: []byte("\tA\tB\tC\n"),
		},
		{
			// R2.2: tab past last explicit stop replaced by single space.
			Name:  "t_list_past_last_stop",
			Args:  []string{"-t", "4,8"},
			Stdin: []byte("a\tb\tc\td\n"),
		},
		{
			// R2.2: explicit positions with --tabs= form.
			Name:  "tabs_list_long_form",
			Args:  []string{"--tabs=4,8,12"},
			Stdin: []byte("\tA\tB\tC\n"),
		},
		{
			// R2.2: explicit positions with short -t form.
			Name:  "t_list_short_form",
			Args:  []string{"-t4,8,12"},
			Stdin: []byte("\tA\tB\tC\n"),
		},
		// R2.4: single value in list behaves as uniform interval.
		{
			Name:  "t_single_in_list",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a\tb\tc\n"),
		},
		// R2.4: error on non-increasing tab stops.
		{
			Name:      "t_error_non_increasing",
			Args:      []string{"-t", "4,2"},
			Stdin:     []byte("a\tb\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R2.4: error on zero tab stop.
		{
			Name:      "t_error_zero",
			Args:      []string{"-t", "0"},
			Stdin:     []byte("a\tb\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R2.4: error on negative tab stop.
		{
			Name:      "t_error_negative",
			Args:      []string{"-t", "-1"},
			Stdin:     []byte("a\tb\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R2.4: error on non-numeric tab stop.
		{
			Name:      "t_error_non_numeric",
			Args:      []string{"-t", "abc"},
			Stdin:     []byte("a\tb\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		{
			// R2.2: tab at exact stop position advances to next.
			Name:  "t_list_tab_at_stop",
			Args:  []string{"-t", "4,8,12"},
			Stdin: []byte("abc\tdef\tghi\tjkl\n"),
		},
		{
			// R2.1: file input with custom tab stop.
			Name: "t_uniform_with_file",
			Args: []string{"-t", "4", tabFile},
		},
		// R3.1: -i flag converts only leading tabs, leaving embedded tabs unchanged.
		{
			Name:  "initial_only_basic",
			Args:  []string{"-i"},
			Stdin: []byte("\thello\tworld\n"),
		},
		{
			// R3.1: -i with multiple leading tabs.
			Name:  "initial_only_multiple_leading",
			Args:  []string{"-i"},
			Stdin: []byte("\t\thello\tworld\n"),
		},
		{
			// R3.1: -i with no leading tabs — all tabs are embedded, none expanded.
			Name:  "initial_only_no_leading_tabs",
			Args:  []string{"-i"},
			Stdin: []byte("hello\tworld\n"),
		},
		{
			// R3.1: -i with only leading tabs and no embedded tabs.
			Name:  "initial_only_leading_only",
			Args:  []string{"-i"},
			Stdin: []byte("\t\thello\n"),
		},
		{
			// R3.1: -i with leading spaces and tabs (spaces are blank, don't end initial).
			Name:  "initial_only_leading_space_tab",
			Args:  []string{"-i"},
			Stdin: []byte(" \thello\tworld\n"),
		},
		{
			// R3.1: -i across multiple lines — each line resets initial state.
			Name:  "initial_only_multiline",
			Args:  []string{"-i"},
			Stdin: []byte("\thello\tworld\n\tfoo\tbar\n"),
		},
		{
			// R3.1: -i with --initial long form.
			Name:  "initial_long_form",
			Args:  []string{"--initial"},
			Stdin: []byte("\thello\tworld\n"),
		},
		{
			// R3.1: -i with tab-only line.
			Name:  "initial_only_tabs_only",
			Args:  []string{"-i"},
			Stdin: []byte("\t\t\t\n"),
		},
		{
			// R3.1: -i with empty input.
			Name:  "initial_only_empty",
			Args:  []string{"-i"},
			Stdin: []byte{},
		},
		// R3.2: -i combined with custom tab stops.
		{
			Name:  "initial_with_t4",
			Args:  []string{"-i", "-t", "4"},
			Stdin: []byte("\thello\tworld\n"),
		},
		{
			Name:  "initial_with_t_list",
			Args:  []string{"-i", "-t", "4,8,12"},
			Stdin: []byte("\t\thello\tworld\n"),
		},
		{
			// R3.2: -i with custom tab stops, multiple leading tabs.
			Name:  "initial_with_t2_multiple_leading",
			Args:  []string{"-i", "-t", "2"},
			Stdin: []byte("\t\t\thello\tworld\n"),
		},
		{
			// R3.2: -i with custom tab stop list and no embedded tabs.
			Name:  "initial_with_t_list_no_embedded",
			Args:  []string{"-i", "-t", "5,10"},
			Stdin: []byte("\t\thello\n"),
		},
		// R3.3: stdin reading when no file arguments or '-' specified.
		{
			Name:  "stdin_no_args",
			Stdin: []byte("from\tstdin\n"),
		},
		{
			Name:  "stdin_dash_explicit",
			Args:  []string{"-"},
			Stdin: []byte("dash\tstdin\n"),
		},
		// R3.4: multiple file arguments processed as concatenated stream.
		{
			Name: "multifile_concatenation",
			Args: []string{fileA, fileB},
		},
		{
			// R3.4: file and stdin interleaved.
			Name:  "multifile_with_stdin",
			Args:  []string{fileA, "-", fileB},
			Stdin: []byte("stdin\tline\n"),
		},
		{
			// R3.1: -i with file input.
			Name: "initial_with_file",
			Args: []string{"-i", initialFile},
		},

		// --- R4.1: Error handling differential tests ---
		{
			// R4.1: unreadable file produces stderr and exit code 1.
			Name:      "error_nonexistent_file",
			Args:      []string{nonexistentFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer, stderrCaseNormalizer},
		},
		{
			// R4.1: nonexistent file among valid files — processing continues,
			// stdout from valid files still produced.
			Name:      "error_nonexistent_with_valid",
			Args:      []string{tabFile, nonexistentFile, noTabFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer, stderrCaseNormalizer},
		},
		{
			// R4.1: -t with non-increasing tab stop list.
			Name:      "error_t_non_increasing_list",
			Args:      []string{"-t", "8,4"},
			Stdin:     []byte("a\tb\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		{
			// R4.1: -t with non-numeric value.
			Name:      "error_t_non_numeric_value",
			Args:      []string{"-t", "foo"},
			Stdin:     []byte("a\tb\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		{
			// R4.1: -t with zero tab stop.
			Name:      "error_t_zero_value",
			Args:      []string{"-t", "0"},
			Stdin:     []byte("a\tb\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},

		// --- R4.2: Edge case differential tests ---
		{
			// R4.2: empty file produces no output.
			Name: "edge_empty_file",
			Args: []string{emptyFile},
		},
		{
			// R4.2: file containing only tabs.
			Name: "edge_tab_only_file",
			Args: []string{tabOnlyFile},
		},
		{
			// R4.2: file with no tabs passes through unchanged.
			Name: "edge_no_tab_file",
			Args: []string{noTabFile},
		},
		{
			// R4.2: very long line with interleaved tabs.
			Name: "edge_very_long_line_file",
			Args: []string{longLineFile},
		},
		{
			// R4.2: very long line via stdin.
			Name:  "edge_very_long_line_stdin",
			Stdin: []byte(strings.Repeat("x\t", 500) + "\n"),
		},
		{
			// R4.2: mixed line endings (CRLF) — \r is a non-tab byte, passed through.
			Name:  "edge_mixed_crlf",
			Stdin: []byte("a\tb\r\nc\td\r\n"),
		},
		{
			// R4.2: carriage return without newline.
			Name:  "edge_cr_only",
			Stdin: []byte("a\tb\rc\td\r"),
		},
		{
			// R4.2: binary content with null bytes.
			Name: "edge_binary_file",
			Args: []string{binaryFile},
		},
		{
			// R4.2: binary content via stdin with null bytes and tabs.
			Name:  "edge_binary_stdin",
			Stdin: []byte("\x00\t\x01\t\x02\n"),
		},
		{
			// R4.2: input with only newlines (no tabs).
			Name:  "edge_only_newlines",
			Stdin: []byte("\n\n\n\n\n"),
		},
		{
			// R4.2: single tab with no newline.
			Name:  "edge_single_tab_no_newline",
			Stdin: []byte("\t"),
		},
		{
			// R4.2: tab-only input with no newline.
			Name:  "edge_tabs_no_newline",
			Stdin: []byte("\t\t\t"),
		},
		{
			// R4.2: alternating tabs and spaces.
			Name:  "edge_alternating_tab_space",
			Stdin: []byte("\t \t \t \n"),
		},

		// --- R4.3: Flag combination differential tests ---
		{
			// R4.3: -i with -t N (custom uniform interval).
			Name:  "combo_i_t_uniform",
			Args:  []string{"-i", "-t", "4"},
			Stdin: []byte("\t\thello\tworld\n"),
		},
		{
			// R4.3: -i with -t LIST (custom explicit positions).
			Name:  "combo_i_t_list",
			Args:  []string{"-i", "-t", "4,8,12"},
			Stdin: []byte("\t\thello\tworld\n"),
		},
		{
			// R4.3: -t with multiple file arguments.
			Name: "combo_t_multiple_files",
			Args: []string{"-t", "4", fileA, fileB},
		},
		{
			// R4.3: -t LIST with multiple file arguments.
			Name: "combo_t_list_multiple_files",
			Args: []string{"-t", "4,8", tabFile, fileA, fileB},
		},
		{
			// R4.3: -i with -t and multiple files.
			Name: "combo_i_t_multiple_files",
			Args: []string{"-i", "-t", "4", initialFile, tabFile},
		},
		{
			// R4.3: -t with file and stdin interleaved.
			Name:  "combo_t_file_and_stdin",
			Args:  []string{"-t", "4", fileA, "-", fileB},
			Stdin: []byte("\tstdin\tline\n"),
		},
		{
			// R4.3: -i with -t and file and stdin interleaved.
			Name:  "combo_i_t_file_and_stdin",
			Args:  []string{"-i", "-t", "4", fileA, "-"},
			Stdin: []byte("\thello\tworld\n"),
		},
		{
			// R4.3: -i combined with -t LIST and multiline input.
			Name:  "combo_i_t_list_multiline",
			Args:  []string{"-i", "-t", "4,8"},
			Stdin: []byte("\thello\tworld\n\t\tfoo\tbar\n"),
		},
		{
			// R4.3: -i with -t 1 (minimum interval).
			Name:  "combo_i_t1_minimum",
			Args:  []string{"-i", "-t", "1"},
			Stdin: []byte("\t\thello\tworld\n"),
		},
		{
			// R4.3: --initial --tabs=N long form combination.
			Name:  "combo_initial_tabs_long_form",
			Args:  []string{"--initial", "--tabs=4"},
			Stdin: []byte("\thello\tworld\n"),
		},
		{
			// R4.3: -i with -t large value.
			Name:  "combo_i_t_large",
			Args:  []string{"-i", "-t", "20"},
			Stdin: []byte("\thello\tworld\n"),
		},
		{
			// R4.3: -t with empty file among other files.
			Name: "combo_t_with_empty_file",
			Args: []string{"-t", "4", emptyFile, tabFile},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
