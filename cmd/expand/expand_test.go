// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/expand against the GNU reference binary (gexpand).
// Implements prd024-expand R1.1-R1.4, R2.1-R2.4 test coverage.
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
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
