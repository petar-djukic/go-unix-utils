// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cat against gcat (GNU coreutils).
//
// Covers prd006-cat R2.4, R3.1, R3.2, R3.3, R4.1, R4.2, R4.3, R4.4.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcat")
	if err != nil {
		t.Skip("reference binary gcat not in PATH")
	}

	tests := []testutils.DiffTest{
		// R2.4: blank line is only \n; lines with spaces/tabs are not blank
		{
			Name:  "R2.4_blank_vs_whitespace_with_b",
			Args:  []string{"-b"},
			Stdin: []byte("a\n\n \n\t\nb\n"),
		},
		// R3.1: suppress repeated blank lines
		{
			Name:  "R3.1_squeeze_repeated_blanks",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		{
			Name:  "R3.1_squeeze_single_blank_kept",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\nb\n"),
		},
		{
			Name:  "R3.1_squeeze_three_blanks",
			Args:  []string{"-s"},
			Stdin: []byte("\n\n\na\n\n\n\nb\n\n\n"),
		},
		// R3.2: squeeze across file boundaries
		{
			Name:     "R3.2_squeeze_across_files",
			Args:     []string{"-s"},
			ExitCode: 0,
		},
		// R3.3: squeeze with -n (squeeze before numbering)
		{
			Name:  "R3.3_squeeze_with_n",
			Args:  []string{"-sn"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		// R3.3: squeeze with -b
		{
			Name:  "R3.3_squeeze_with_b",
			Args:  []string{"-sb"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		// R2.4: ensure spaces-only line is not blank for -b
		{
			Name:  "R2.4_space_line_numbered_by_b",
			Args:  []string{"-b"},
			Stdin: []byte("a\n \nb\n"),
		},
		// R3.1: no blanks, no effect
		{
			Name:  "R3.1_no_blanks_no_effect",
			Args:  []string{"-s"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R3.3: squeeze with both -n and -b (b overrides n)
		{
			Name:  "R3.3_squeeze_with_bn",
			Args:  []string{"-sbn"},
			Stdin: []byte("x\n\n\n\ny\n"),
		},
		// R4.1: -v shows control characters as ^X
		{
			Name:  "R4.1_v_control_chars",
			Args:  []string{"-v"},
			Stdin: []byte{0x01, 0x02, 0x1B, 0x0A},
		},
		// R4.1: -v shows DEL as ^?
		{
			Name:  "R4.1_v_del_char",
			Args:  []string{"-v"},
			Stdin: []byte{0x7F, 0x0A},
		},
		// R4.1: -v shows high bytes with M- prefix
		{
			Name:  "R4.1_v_high_bytes",
			Args:  []string{"-v"},
			Stdin: []byte{0x80, 0x9F, 0xA0, 0xFE, 0xFF, 0x0A},
		},
		// R4.1: -v with full range of control chars
		{
			Name:  "R4.1_v_all_low_controls",
			Args:  []string{"-v"},
			Stdin: []byte{0x00, 0x03, 0x07, 0x08, 0x0E, 0x1F, 0x0A},
		},
		// R4.2: -v does not alter tab or newline
		{
			Name:  "R4.2_v_preserves_tab_newline",
			Args:  []string{"-v"},
			Stdin: []byte("hello\tworld\n"),
		},
		// R4.3: -E shows $ at end of line
		{
			Name:  "R4.3_E_dollar_at_eol",
			Args:  []string{"-E"},
			Stdin: []byte("hello\nworld\n"),
		},
		// R4.3: -E with blank lines
		{
			Name:  "R4.3_E_blank_lines",
			Args:  []string{"-E"},
			Stdin: []byte("a\n\n\nb\n"),
		},
		// R4.4: -T shows tabs as ^I
		{
			Name:  "R4.4_T_tab_as_caret_I",
			Args:  []string{"-T"},
			Stdin: []byte("a\tb\tc\n"),
		},
		// R4.4: -T with multiple tabs
		{
			Name:  "R4.4_T_multiple_tabs",
			Args:  []string{"-T"},
			Stdin: []byte("\t\thello\t\n"),
		},
		// R4.1+R4.3: -v -E combined
		{
			Name:  "R4.1_R4.3_v_E_combined",
			Args:  []string{"-vE"},
			Stdin: []byte{0x01, 'h', 'i', 0x0A},
		},
		// R4.1+R4.4: -v -T combined
		{
			Name:  "R4.1_R4.4_v_T_combined",
			Args:  []string{"-vT"},
			Stdin: []byte{0x01, 0x09, 'a', 0x0A},
		},
		// R4.3+R4.4: -E -T combined
		{
			Name:  "R4.3_R4.4_E_T_combined",
			Args:  []string{"-ET"},
			Stdin: []byte("a\tb\n"),
		},
		// R4.1+R4.3+R4.4: all three combined
		{
			Name:  "R4.1_R4.3_R4.4_all_combined",
			Args:  []string{"-vET"},
			Stdin: []byte{0x01, 0x09, 'x', 0x7F, 0x0A},
		},
		// R4.1: -v with printable ASCII is unchanged
		{
			Name:  "R4.1_v_printable_unchanged",
			Args:  []string{"-v"},
			Stdin: []byte("Hello, World! 123\n"),
		},
	}

	// R3.2: create temp files for cross-boundary squeeze test
	setupCrossBoundaryTest(t, tests)

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// setupCrossBoundaryTest creates temp files for the R3.2 cross-boundary test.
func setupCrossBoundaryTest(t *testing.T, tests []testutils.DiffTest) {
	t.Helper()
	for i := range tests {
		if tests[i].Name != "R3.2_squeeze_across_files" {
			continue
		}
		dir := t.TempDir()
		file1 := filepath.Join(dir, "f1.txt")
		file2 := filepath.Join(dir, "f2.txt")
		writeTestFile(t, file1, "a\n\n")
		writeTestFile(t, file2, "\nb\n")
		tests[i].Args = []string{"-s", file1, file2}
		tests[i].Stdin = nil
	}
}

// writeTestFile writes content to a file, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeTestFile %s: %v", path, err)
	}
}
