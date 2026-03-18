// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd006-cat R1.5, R2.1–R2.4, R3.1–R3.3, R4.1–R4.4.
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

		// R4.1: -v displays control characters as ^X.
		{
			Name:  "show_nonprint_control",
			Args:  []string{"-v"},
			Stdin: []byte{0x01, 0x02, 0x1B, '\n'},
			Env:   []string{"LC_ALL=C"},
		},
		// R4.1: -v displays DEL (0x7F) as ^?.
		{
			Name:  "show_nonprint_del",
			Args:  []string{"-v"},
			Stdin: []byte{0x7F, '\n'},
			Env:   []string{"LC_ALL=C"},
		},
		// R4.1: -v displays high bytes (0x80-0x9F) as M-^X.
		{
			Name:  "show_nonprint_high_control",
			Args:  []string{"-v"},
			Stdin: []byte{0x80, 0x81, 0x9F, '\n'},
			Env:   []string{"LC_ALL=C"},
		},
		// R4.1: -v displays high bytes (0xA0-0xFE) as M-X.
		{
			Name:  "show_nonprint_high_printable",
			Args:  []string{"-v"},
			Stdin: []byte{0xA0, 0xA1, 0xFE, '\n'},
			Env:   []string{"LC_ALL=C"},
		},
		// R4.1: -v displays 0xFF as M-^?.
		{
			Name:  "show_nonprint_0xff",
			Args:  []string{"-v"},
			Stdin: []byte{0xFF, '\n'},
			Env:   []string{"LC_ALL=C"},
		},
		// R4.2: -v does not alter tab or newline.
		{
			Name:  "show_nonprint_preserves_tab_newline",
			Args:  []string{"-v"},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.3: -E appends $ before each newline.
		{
			Name:  "show_ends",
			Args:  []string{"-E"},
			Stdin: []byte("hello\nworld\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.3: -E with blank lines.
		{
			Name:  "show_ends_blank_lines",
			Args:  []string{"-E"},
			Stdin: []byte("a\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.4: -T displays tab as ^I.
		{
			Name:  "show_tabs",
			Args:  []string{"-T"},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.5: -A = -vET combined.
		{
			Name:  "show_all",
			Args:  []string{"-A"},
			Stdin: []byte("a\tb\n\x01\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.6: -e = -vE combined.
		{
			Name:  "show_nonprint_ends",
			Args:  []string{"-e"},
			Stdin: []byte("a\tb\n\x01\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.7: -t = -vT combined.
		{
			Name:  "show_nonprint_tabs",
			Args:  []string{"-t"},
			Stdin: []byte("a\tb\n\x01\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.9: -v combined with -n (number + nonprint).
		{
			Name:  "nonprint_with_number",
			Args:  []string{"-vn"},
			Stdin: []byte("\x01\n\x02\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.9: -A combined with -s (squeeze + all display).
		{
			Name:  "show_all_with_squeeze",
			Args:  []string{"-As"},
			Stdin: []byte("a\n\n\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.9: -A combined with -b (nonblank number + all display).
		{
			Name:  "show_all_with_number_nonblank",
			Args:  []string{"-Ab"},
			Stdin: []byte("a\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.1: full byte range with -v: NUL through 0x1F control chars.
		{
			Name: "show_nonprint_full_control_range",
			Args: []string{"-v"},
			Stdin: []byte{
				0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
				0x08, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
				0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
				0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F,
				'\n',
			},
			Env: []string{"LC_ALL=C"},
		},
		// R4.3: -E with no trailing newline.
		{
			Name:  "show_ends_no_trailing_newline",
			Args:  []string{"-E"},
			Stdin: []byte("hello"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.4: -T with multiple tabs.
		{
			Name:  "show_tabs_multiple",
			Args:  []string{"-T"},
			Stdin: []byte("\t\t\n"),
			Env:   []string{"LC_ALL=C"},
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
