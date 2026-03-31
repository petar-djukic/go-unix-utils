// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/fmt.
// Covers prd070-fmt R1.1, R2.1, R3.1, R4.1, R5.1, R6.1, R7.1, R8.1, R9.1, R10.1, R11.1, R12.1,
// R13.1, R13.2, R13.3, R13.4.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeNonEmpty replaces any non-empty output with a fixed marker.
// Used for stderr where message format differs between Go and GNU.
func normalizeNonEmpty(b []byte) []byte {
	if len(b) > 0 {
		return []byte("ERROR\n")
	}
	return b
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfmt")
	if err != nil {
		t.Skip("reference binary gfmt not in PATH")
	}

	tmpDir := t.TempDir()
	writeTestFile(t, tmpDir, "para.txt",
		"This is a short paragraph that fits in 75 chars easily.\n")
	writeTestFile(t, tmpDir, "multi.txt",
		"first file content here\n")
	writeTestFile(t, tmpDir, "multi2.txt",
		"second file content here\n")

	longLine := "word " +
		"word word word word word word word word word " +
		"word word word word word word word word word " +
		"word word word word word word word word word " +
		"word word word word word word word word word end"

	indented := "    first indented line that is long enough to require wrapping to test indentation preservation properly\n" +
		"        second line has deeper indent and also needs to be long enough to wrap around\n"

	tests := []testutils.DiffTest{
		{
			// R1.1: default 75-char formatting, short line unchanged
			Name:  "short_line_unchanged",
			Args:  []string{},
			Stdin: []byte("short line\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: default 75-char formatting on long line
			Name:  "default_width_wrap",
			Args:  []string{},
			Stdin: []byte(longLine + "\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.1: blank lines separate paragraphs
			Name:  "paragraph_boundary",
			Args:  []string{},
			Stdin: []byte("first paragraph words words words words words\n\nsecond paragraph words words words words words\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.1: multiple blank lines
			Name:  "multiple_blank_lines",
			Args:  []string{},
			Stdin: []byte("para one\n\n\npara two\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R3.1: indentation preservation
			Name:  "indentation_preserved",
			Args:  []string{},
			Stdin: []byte(indented),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R3.1: single-line paragraph indent
			Name:  "single_line_indent",
			Args:  []string{},
			Stdin: []byte("   indented single line\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.1: read from file argument
			Name:  "file_input",
			Args:  []string{filepath.Join(tmpDir, "para.txt")},
			Stdin: nil,
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.1: read from multiple files
			Name: "multi_file_input",
			Args: []string{
				filepath.Join(tmpDir, "multi.txt"),
				filepath.Join(tmpDir, "multi2.txt"),
			},
			Stdin: nil,
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.1: "-" means stdin
			Name:  "dash_stdin",
			Args:  []string{"-"},
			Stdin: []byte("stdin input line\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.1: missing file produces error exit code
			Name:      "missing_file_error",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			Stdin:     nil,
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeNonEmpty},
		},
		{
			// R5.1: -w sets custom width
			Name:  "width_short_flag",
			Args:  []string{"-w", "30"},
			Stdin: []byte("this is a line of text that should be wrapped at thirty characters\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R5.1: --width=N long form
			Name:  "width_long_equals",
			Args:  []string{"--width=40"},
			Stdin: []byte("this is a line of text that should be wrapped at about forty characters or so\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R5.1: --width N space-separated long form
			Name:  "width_long_space",
			Args:  []string{"--width", "30"},
			Stdin: []byte("this is a line of text that should be wrapped at thirty characters\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R5.1: -wN attached value
			Name:  "width_attached",
			Args:  []string{"-w30"},
			Stdin: []byte("this is a line of text that should be wrapped at thirty characters\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R6.1: -g sets goal width
			Name:  "goal_width",
			Args:  []string{"-w", "60", "-g", "50"},
			Stdin: []byte("this is some text that we want to fill to approximately fifty characters per line in the output\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R6.1: --goal=N long form
			Name:  "goal_long_equals",
			Args:  []string{"--goal=20", "-w", "30"},
			Stdin: []byte("some text here that needs formatting with a goal width setting\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R7.1: long word exceeding width is not broken
			Name:  "long_word_no_break",
			Args:  []string{"-w", "10"},
			Stdin: []byte("a verylongwordthatexceedswidth b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R8.1: sentence-ending punctuation gets two spaces
			Name:  "sentence_end_spacing",
			Args:  []string{},
			Stdin: []byte("First sentence.  Second sentence.  Third sentence.\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R8.1: multiple spaces in short line pass through (GNU fmt behavior)
			Name:  "space_preserve_short",
			Args:  []string{},
			Stdin: []byte("word     word    word\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R8.1: original spacing preserved when reformatting
			Name:  "space_preserve_reformat",
			Args:  []string{"-w", "20"},
			Stdin: []byte("lots   of   extra   spaces   here\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R8.1: single-space text reformatted at custom width
			Name:  "reformat_single_space",
			Args:  []string{"-w", "30"},
			Stdin: []byte("End of sentence. Start of another sentence. And more text here.\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R9.1: split-only mode
		{
			Name:  "split_only_short_unchanged",
			Args:  []string{"-s"},
			Stdin: []byte("short line\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "split_only_long_split",
			Args:  []string{"-s", "-w", "20"},
			Stdin: []byte("this is a line that is definitely longer than twenty characters\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "split_only_no_join",
			Args:  []string{"-s"},
			Stdin: []byte("short\nlines\nare\nnot\njoined\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "split_only_blank_lines",
			Args:  []string{"-s"},
			Stdin: []byte("line one\n\nline two\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R10.1: uniform spacing
		{
			Name:  "uniform_spacing_normalize",
			Args:  []string{"-u", "-w", "60"},
			Stdin: []byte("word   word    word    word word word word word word word word word word word word\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "uniform_spacing_sentence_end",
			Args:  []string{"-u"},
			Stdin: []byte("End of sentence. Start of next. More text here to make this line long enough to need reformatting by the formatter.\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "uniform_spacing_short_line",
			Args:  []string{"-u"},
			Stdin: []byte("word    word     word\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R11.1: prefix mode
		{
			Name:  "prefix_short_passthrough",
			Args:  []string{"-p", "> "},
			Stdin: []byte("> short line\nnot prefixed\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "prefix_long_wrap",
			Args:  []string{"-p", "> ", "-w", "30"},
			Stdin: []byte("> This line is long enough that it must be wrapped at thirty characters.\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "prefix_long_flag",
			Args:  []string{"--prefix=> "},
			Stdin: []byte("> hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R12.1: tagged-paragraph mode
		{
			// First line forced separate (next word exceeds width).
			Name:  "tagged_paragraph_mode",
			Args:  []string{"-t", "-w", "40"},
			Stdin: []byte("  First line fills up the line here.\n    Body text is long enough to require\n    wrapping around.\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "tagged_paragraph_long_flag",
			Args:  []string{"--tagged-paragraph", "-w", "40"},
			Stdin: []byte("Header line for paragraph.\n  Body text with indent that should be preserved and wrapped if needed.\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R13.1: exit 0 on successful formatting (covered by all tests above with ExitCode 0)
		// R13.2: exit 1 on invalid option
		{
			Name:      "invalid_option_error",
			Args:      []string{"--bogus"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeNonEmpty},
		},
		// R13.2: exit 1 on invalid width value
		{
			Name:      "invalid_width_error",
			Args:      []string{"-w", "abc"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeNonEmpty},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	if err != nil {
		t.Fatalf("writeTestFile %s: %v", name, err)
	}
}
