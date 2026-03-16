// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/expand against the GNU reference binary (gexpand).
// Implements prd024-expand R1.1-R1.4 test coverage.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

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
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
