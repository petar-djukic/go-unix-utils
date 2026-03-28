// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd022-nl R4.1, R4.2, R4.3, R4.4.
// R4.1: compare Go nl against gnl byte-for-byte via RunDiffTests.
// R4.2: cover numbering styles (-b a/t/n/pBRE), format flags (-n ln/rn/rz, -w, -s).
// R4.3: cover section delimiters (-h, -f, -d, -p).
// R4.4: cover edge cases (empty input, -v, -i, -l, multiple files, invalid options).
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
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skip("reference binary gnl not in PATH")
	}

	tests := []testutils.DiffTest{
		// --- R4.1: default behavior ---

		// Default mode: number non-empty lines, right-justified 6-digit, tab separator.
		{
			Name:  "default_body_numbering",
			Stdin: []byte("first\n\nsecond\n"),
		},

		// Only non-empty lines are numbered; empty lines pass through.
		{
			Name:  "default_empty_lines_unnumbered",
			Stdin: []byte("a\n\n\nb\n"),
		},

		// Stdin read when no file arguments given.
		{
			Name:  "stdin_no_args",
			Stdin: []byte("line1\nline2\nline3\n"),
		},

		// Stdin via explicit "-" argument.
		{
			Name:  "stdin_dash_arg",
			Args:  []string{"-"},
			Stdin: []byte("alpha\nbeta\n"),
		},

		// --- R4.2: numbering styles and format flags ---

		// -b a: number all lines including empty.
		{
			Name:  "body_style_all",
			Args:  []string{"-b", "a"},
			Stdin: []byte("x\n\ny\n"),
		},

		// -b t: number non-empty lines (explicit default).
		{
			Name:  "body_style_nonempty",
			Args:  []string{"-b", "t"},
			Stdin: []byte("foo\n\nbar\n"),
		},

		// -b n: number no lines.
		{
			Name:  "body_style_none",
			Args:  []string{"-b", "n"},
			Stdin: []byte("one\ntwo\n"),
		},

		// -b pRE: number lines matching regex.
		{
			Name:  "body_style_regex",
			Args:  []string{"-b", "p^[A-Z]"},
			Stdin: []byte("Hello\nworld\nGoodbye\n"),
		},

		// -n ln: left-justified numbering.
		{
			Name:  "format_left_justified",
			Args:  []string{"-n", "ln"},
			Stdin: []byte("a\nb\n"),
		},

		// -n rn: right-justified (default, explicit).
		{
			Name:  "format_right_justified",
			Args:  []string{"-n", "rn"},
			Stdin: []byte("a\nb\n"),
		},

		// -n rz: right-justified with leading zeros.
		{
			Name:  "format_right_zero",
			Args:  []string{"-n", "rz"},
			Stdin: []byte("a\nb\n"),
		},

		// -w 3: custom field width.
		{
			Name:  "width_3",
			Args:  []string{"-w", "3"},
			Stdin: []byte("a\nb\nc\n"),
		},

		// -n ln -w 3 -s ': ': combined format, width, separator.
		{
			Name:  "format_ln_width_3_separator",
			Args:  []string{"-n", "ln", "-w", "3", "-s", ": "},
			Stdin: []byte("a\nb\n"),
		},

		// -s '|': custom separator.
		{
			Name:  "custom_separator_pipe",
			Args:  []string{"-s", "|"},
			Stdin: []byte("first\nsecond\n"),
		},

		// -v 10 -i 5: start at 10, increment by 5.
		{
			Name:  "start_and_increment",
			Args:  []string{"-v", "10", "-i", "5"},
			Stdin: []byte("p\nq\nr\n"),
		},

		// -b a -n rz -w 4: all lines, zero-padded, width 4.
		{
			Name:  "all_lines_rz_w4",
			Args:  []string{"-b", "a", "-n", "rz", "-w", "4"},
			Stdin: []byte("a\n\nb\n"),
		},

		// --- R4.3: section delimiters ---

		// Header delimiter resets counter; -h a numbers header lines.
		{
			Name:  "section_header_body",
			Args:  []string{"-h", "a"},
			Stdin: []byte("\\:\\:\\:\nheader line\n\\:\\:\nbody line\n"),
		},

		// Footer section with -f a.
		{
			Name:  "section_footer",
			Args:  []string{"-f", "a"},
			Stdin: []byte("body1\n\\:\nfooter1\n"),
		},

		// Full three-section logical page.
		{
			Name:  "section_full_page",
			Args:  []string{"-h", "a", "-f", "a"},
			Stdin: []byte("\\:\\:\\:\nH1\n\\:\\:\nB1\nB2\n\\:\nF1\n"),
		},

		// -p: suppress counter reset on new logical page.
		{
			Name:  "no_reset_p_flag",
			Args:  []string{"-p", "-h", "a"},
			Stdin: []byte("line1\n\\:\\:\\:\nheader\n\\:\\:\nbody\n"),
		},

		// -d XX: custom two-character delimiter.
		{
			Name:  "custom_delimiter_d",
			Args:  []string{"-d", "XX", "-h", "a"},
			Stdin: []byte("XXXXXX\nheader\nXXXX\nbody\n"),
		},

		// -d X: single-character delimiter padded with ':'.
		{
			Name:  "single_char_delimiter",
			Args:  []string{"-d", "X", "-h", "a"},
			Stdin: []byte("X:X:X:\nheader\nX:X:\nbody\n"),
		},

		// Multiple logical pages: counter resets on each header.
		{
			Name:  "multiple_pages_reset",
			Args:  []string{"-h", "a"},
			Stdin: []byte("\\:\\:\\:\nH1\n\\:\\:\nB1\n\\:\\:\\:\nH2\n\\:\\:\nB2\n"),
		},

		// Multiple pages with -p: counter does NOT reset.
		{
			Name:  "multiple_pages_no_reset",
			Args:  []string{"-p", "-h", "a"},
			Stdin: []byte("\\:\\:\\:\nH1\n\\:\\:\nB1\n\\:\\:\\:\nH2\n\\:\\:\nB2\n"),
		},

		// --- R4.4: edge cases ---

		// Empty input produces no output.
		{
			Name:  "empty_input",
			Stdin: []byte(""),
		},

		// Single line with trailing newline.
		{
			Name:  "single_line",
			Stdin: []byte("only\n"),
		},

		// -v 100: start numbering at 100.
		{
			Name:  "start_at_100",
			Args:  []string{"-v", "100"},
			Stdin: []byte("a\nb\n"),
		},

		// -i 10: increment by 10.
		{
			Name:  "increment_10",
			Args:  []string{"-i", "10"},
			Stdin: []byte("a\nb\nc\n"),
		},

		// -l 2: join blank count of 2; two empty lines treated as one unnumbered block.
		{
			Name:  "join_blank_l2",
			Args:  []string{"-b", "a", "-l", "2"},
			Stdin: []byte("a\n\nb\n\n\nc\n"),
		},

		// -l 3: three empty lines needed before numbering an empty line.
		{
			Name:  "join_blank_l3",
			Args:  []string{"-b", "a", "-l", "3"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},

		// -l 1 (default): each empty line counted separately with -b a.
		{
			Name:  "join_blank_l1_default",
			Args:  []string{"-b", "a", "-l", "1"},
			Stdin: []byte("a\n\n\nb\n"),
		},

		// Many consecutive empty lines.
		{
			Name:  "many_empty_lines",
			Stdin: []byte("\n\n\n\n\n"),
		},

		// Lines with only whitespace (not truly empty per nl).
		{
			Name:  "whitespace_only_lines",
			Stdin: []byte("  \n\t\n\n"),
		},

		// Binary data (null bytes).
		{
			Name:  "binary_data",
			Stdin: []byte("line1\x00embedded\nline2\n"),
		},

		// Long line.
		{
			Name:  "long_line",
			Stdin: []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789\n"),
		},

		// -b a -n rz -w 8 -s ' ' -v 0 -i 1: combined options.
		{
			Name:  "combined_options",
			Args:  []string{"-b", "a", "-n", "rz", "-w", "8", "-s", " ", "-v", "0", "-i", "1"},
			Stdin: []byte("first\n\nsecond\n"),
		},

	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffMultiFile tests nl with multiple file arguments.
// R4.4: line numbering is continuous across files.
func TestDiffMultiFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skip("reference binary gnl not in PATH")
	}

	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "f1.txt")
	file2 := filepath.Join(tmpDir, "f2.txt")
	writeTestFile(t, file1, "alpha\nbeta\n")
	writeTestFile(t, file2, "gamma\ndelta\n")

	tests := []testutils.DiffTest{
		// Continuous numbering across two files.
		{
			Name: "multi_file_continuous",
			Args: []string{file1, file2},
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
