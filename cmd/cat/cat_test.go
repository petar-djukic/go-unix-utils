// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cat against gcat (GNU coreutils).
// Implements prd006-cat R1.5, R2.1-R2.4, R3.1-R3.3, R4.1-R4.9 test coverage.
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
	writeTestFile(t, tmpDir, "trailing-blanks.txt", "x\n\n\n")
	writeTestFile(t, tmpDir, "leading-blanks.txt", "\n\ny\n")

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

		// R2.4: tabs-only line is not blank (contains non-newline bytes).
		{
			Name:  "R2.4_tab_not_blank",
			Args:  []string{"-b"},
			Stdin: []byte("a\n\t\n\nb\n"),
		},

		// R3.1: -s suppresses repeated blank lines.
		{
			Name:  "R3.1_squeeze_blanks",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		// R3.1: -s single blank line passes through.
		{
			Name:  "R3.1_single_blank_passes",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\nb\n"),
		},
		// R3.1: -s leading blank lines squeezed.
		{
			Name:  "R3.1_leading_blanks",
			Args:  []string{"-s"},
			Stdin: []byte("\n\n\na\n"),
		},
		// R3.1: -s trailing blank lines squeezed.
		{
			Name:  "R3.1_trailing_blanks",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\n\n"),
		},
		// R3.1: -s spaces-only line is not blank.
		{
			Name:  "R3.1_spaces_not_blank",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\n \n\nb\n"),
		},

		// R3.2: -s applies across file boundaries.
		{
			Name: "R3.2_squeeze_across_files",
			Args: []string{
				"-s",
				filepath.Join(tmpDir, "trailing-blanks.txt"),
				filepath.Join(tmpDir, "leading-blanks.txt"),
			},
			WorkDir: tmpDir,
		},

		// R3.3: -s combined with -n — suppressed lines don't consume numbers.
		{
			Name:  "R3.3_squeeze_with_number",
			Args:  []string{"-sn"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		// R3.3: -s combined with -b — squeeze before numbering.
		{
			Name:  "R3.3_squeeze_with_number_nonblank",
			Args:  []string{"-sb"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},

		// R4.1: -v displays control characters with caret notation.
		{
			Name:  "R4.1_v_control_chars",
			Args:  []string{"-v"},
			Stdin: []byte{0x01, 0x02, 0x03, 0x1B, '\n'},
		},
		// R4.1: -v displays DEL as ^?.
		{
			Name:  "R4.1_v_del",
			Args:  []string{"-v"},
			Stdin: []byte{0x7F, '\n'},
		},
		// R4.1: -v displays high bytes with M- prefix.
		{
			Name:  "R4.1_v_high_bytes",
			Args:  []string{"-v"},
			Stdin: []byte{0x80, 0x9F, 0xA0, 0xFE, 0xFF, '\n'},
		},
		// R4.1: -v mixed printable and non-printing.
		{
			Name:  "R4.1_v_mixed",
			Args:  []string{"-v"},
			Stdin: []byte("hello\x01world\x7f\n"),
		},

		// R4.2: -v does not alter tab or newline.
		{
			Name:  "R4.2_v_preserves_tab_newline",
			Args:  []string{"-v"},
			Stdin: []byte("a\tb\n"),
		},

		// R4.3: -E appends "$" before newlines.
		{
			Name:  "R4.3_E_show_ends",
			Args:  []string{"-E"},
			Stdin: []byte("hello\nworld\n"),
		},
		// R4.3: -E with blank lines.
		{
			Name:  "R4.3_E_blank_lines",
			Args:  []string{"-E"},
			Stdin: []byte("a\n\n\nb\n"),
		},
		// R4.3: -E no trailing newline.
		{
			Name:  "R4.3_E_no_trailing_newline",
			Args:  []string{"-E"},
			Stdin: []byte("abc"),
		},

		// R4.4: -T displays tabs as ^I.
		{
			Name:  "R4.4_T_show_tabs",
			Args:  []string{"-T"},
			Stdin: []byte("a\tb\tc\n"),
		},
		// R4.4: -T with multiple tabs.
		{
			Name:  "R4.4_T_multiple_tabs",
			Args:  []string{"-T"},
			Stdin: []byte("\t\thello\t\n"),
		},

		// R4.5: -A combines -v -E -T.
		{
			Name:  "R4.5_A_all",
			Args:  []string{"-A"},
			Stdin: []byte("a\tb\x01\n"),
		},

		// R4.6: -e combines -v -E.
		{
			Name:  "R4.6_e_nonprinting_ends",
			Args:  []string{"-e"},
			Stdin: []byte("a\tb\x01\n"),
		},

		// R4.7: -t combines -v -T.
		{
			Name:  "R4.7_t_nonprinting_tabs",
			Args:  []string{"-t"},
			Stdin: []byte("a\tb\x01\n"),
		},

		// R4.9: transformation order with -n and -A combined.
		{
			Name:  "R4.9_nA_combined",
			Args:  []string{"-nA"},
			Stdin: []byte("a\tb\x01\n\nc\n"),
		},
		// R4.9: -sbA combined (squeeze + nonprinting + ends + tabs + number nonblank).
		{
			Name:  "R4.9_sbA_combined",
			Args:  []string{"-sbA"},
			Stdin: []byte("a\n\n\n\nb\t\x01\n"),
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
