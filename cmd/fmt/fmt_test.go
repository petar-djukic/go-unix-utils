// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd070-fmt R1.1–R1.4: basic paragraph filling
// and argument parsing.
// Differential tests for prd070-fmt R2.1–R2.4: width control, goal width,
// word breaking, and space collapsing.
// Differential tests for prd070-fmt R4.1–R4.4: exit codes and error cases.
package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfmt")
	if err != nil {
		t.Skipf("reference binary gfmt not in PATH: %v", err)
	}

	tmpDir := t.TempDir()
	singleLine := filepath.Join(tmpDir, "single.txt")
	writeTestFile(t, singleLine, "short line\n")

	longPara := filepath.Join(tmpDir, "long.txt")
	writeTestFile(t, longPara, "This is a long paragraph that should be reformatted to fit within the default width of seventy-five characters when processed by fmt.\n")

	twoParagraphs := filepath.Join(tmpDir, "two_para.txt")
	writeTestFile(t, twoParagraphs, "First paragraph line one. First paragraph line two.\n\nSecond paragraph line one. Second paragraph line two.\n")

	indentedFile := filepath.Join(tmpDir, "indented.txt")
	writeTestFile(t, indentedFile, "    This is an indented paragraph that has a first line with four spaces of indentation and should be reformatted preserving that indentation.\n")

	multiIndent := filepath.Join(tmpDir, "multi_indent.txt")
	writeTestFile(t, multiIndent, "  First line with two spaces.\n    Second line with four spaces and enough text that it wraps around.\n    Third line also with four spaces.\n")

	emptyFile := filepath.Join(tmpDir, "empty.txt")
	writeTestFile(t, emptyFile, "")

	fileA := filepath.Join(tmpDir, "a.txt")
	writeTestFile(t, fileA, "File A content here.\n")
	fileB := filepath.Join(tmpDir, "b.txt")
	writeTestFile(t, fileB, "File B content here.\n")

	blankLines := filepath.Join(tmpDir, "blanks.txt")
	writeTestFile(t, blankLines, "para one\n\n\npara two\n")

	tests := []testutils.DiffTest{
		// R1.1: short line unchanged at default width.
		{
			Name:  "short_line_unchanged",
			Stdin: []byte("short line\n"),
		},
		// R1.1: long paragraph reformatted to 75 chars.
		{
			Name:  "long_paragraph_wrap",
			Stdin: []byte("This is a long paragraph that should be reformatted to fit within the default width of seventy-five characters when processed by the fmt utility.\n"),
		},
		// R1.2: blank line separates paragraphs.
		{
			Name:  "blank_line_paragraph_separator",
			Stdin: []byte("First paragraph.\n\nSecond paragraph.\n"),
		},
		// R1.2: multiple blank lines preserved as separators.
		{
			Name:  "multiple_blank_lines",
			Stdin: []byte("para one\n\n\npara two\n"),
		},
		// R1.3: indentation preserved on first line.
		{
			Name:  "preserve_first_line_indent",
			Stdin: []byte("    Indented paragraph that has a first line with leading spaces and should reformat preserving that indentation across the wrap.\n"),
		},
		// R1.3: subsequent lines use second line's indentation.
		{
			Name:  "subsequent_line_indent",
			Stdin: []byte("  First line with two spaces.\n    Second line with four spaces and enough text to cause wrapping in the output.\n    Third line also with four spaces.\n"),
		},
		// R1.4: stdin with no arguments.
		{
			Name:  "stdin_no_args",
			Stdin: []byte("hello world\n"),
		},
		// R1.4: stdin via explicit "-".
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("hello world\n"),
		},
		// R1.4: single file argument.
		{
			Name: "single_file",
			Args: []string{singleLine},
		},
		// R1.4: long paragraph from file.
		{
			Name: "file_long_paragraph",
			Args: []string{longPara},
		},
		// R1.4: two paragraphs from file.
		{
			Name: "file_two_paragraphs",
			Args: []string{twoParagraphs},
		},
		// R1.4: multiple file arguments.
		{
			Name: "multiple_files",
			Args: []string{fileA, fileB},
		},
		// R1.4: empty file.
		{
			Name: "empty_file",
			Args: []string{emptyFile},
		},
		// R1.3: indented file.
		{
			Name: "file_indented",
			Args: []string{indentedFile},
		},
		// R1.3: multi-indent file.
		{
			Name: "file_multi_indent",
			Args: []string{multiIndent},
		},
		// R1.2: blank lines from file.
		{
			Name: "file_blank_lines",
			Args: []string{blankLines},
		},
		// R4.2: missing file produces error and exit 1.
		{
			Name:      "missing_file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R1.1: empty stdin.
		{
			Name:  "empty_stdin",
			Stdin: []byte(""),
		},
		// R1.1: single word.
		{
			Name:  "single_word",
			Stdin: []byte("word\n"),
		},
		// R1.1: exactly at width boundary.
		{
			Name:  "at_width_boundary",
			Stdin: []byte("aaaaaaaaa bbbbbbbbb ccccccccc ddddddddd eeeeeeeee fffffffff ggggggggg hh\n"),
		},
		// R2.1: -w sets custom width.
		{
			Name:  "width_w_flag",
			Args:  []string{"-w", "40"},
			Stdin: []byte("This is a paragraph that should be reformatted to fit within forty characters width.\n"),
		},
		// R2.1: -w with attached value.
		{
			Name:  "width_w_attached",
			Args:  []string{"-w40"},
			Stdin: []byte("This is a paragraph that should be reformatted to fit within forty characters width.\n"),
		},
		// R2.1: --width= long form.
		{
			Name:  "width_long_equals",
			Args:  []string{"--width=40"},
			Stdin: []byte("This is a paragraph that should be reformatted to fit within forty characters width.\n"),
		},
		// R2.1: -NUMBER shorthand.
		{
			Name:  "width_numeric_shorthand",
			Args:  []string{"-40"},
			Stdin: []byte("This is a paragraph that should be reformatted to fit within forty characters width.\n"),
		},
		// R2.1: very narrow width forces wrapping.
		{
			Name:  "width_narrow",
			Args:  []string{"-w", "20"},
			Stdin: []byte("short words here and there too\n"),
		},
		// R2.1: wide width keeps text on one line.
		{
			Name:  "width_wide",
			Args:  []string{"-w", "200"},
			Stdin: []byte("This is a paragraph that would normally wrap at 75 but should stay on one line with a very wide width setting.\n"),
		},
		// R2.2: -g sets goal width.
		{
			Name:  "goal_g_flag",
			Args:  []string{"-w", "60", "-g", "30"},
			Stdin: []byte("Words here that will be formatted with a narrow goal width for shorter lines.\n"),
		},
		// R2.2: --goal= long form.
		{
			Name:  "goal_long_equals",
			Args:  []string{"-w", "60", "--goal=30"},
			Stdin: []byte("Words here that will be formatted with a narrow goal width for shorter lines.\n"),
		},
		// R2.2: goal defaults to 93% of width.
		{
			Name:  "goal_default_93_percent",
			Args:  []string{"-w", "50"},
			Stdin: []byte("Testing that the goal defaults to ninety three percent of width when not explicitly set.\n"),
		},
		// R2.3: word boundary breaking at narrow width.
		{
			Name:  "word_boundary_narrow",
			Args:  []string{"-w", "15"},
			Stdin: []byte("one two three four five six\n"),
		},
		// R2.3: overlong word exceeding width gets its own line.
		{
			Name:  "overlong_word",
			Args:  []string{"-w", "10"},
			Stdin: []byte("a verylongwordthatexceedswidth b\n"),
		},
		// R2.3: multiple overlong words.
		{
			Name:  "multiple_overlong_words",
			Args:  []string{"-w", "5"},
			Stdin: []byte("longword1 longword2 ok\n"),
		},
		// R2.4: original spacing within a line is preserved.
		{
			Name:  "preserve_same_line_spacing",
			Stdin: []byte("word1   word2     word3    word4\n"),
		},
		// R2.4: sentence-ending punctuation spacing.
		{
			Name:  "sentence_punctuation_spacing",
			Stdin: []byte("End of sentence. Start of next.\nAnother line here.\n"),
		},
		// R2.4: mixed punctuation and multiple spaces.
		{
			Name:  "mixed_punctuation_spaces",
			Stdin: []byte("Hello world.   This is a test!   And more?   Final words.\nNext line begins here.\n"),
		},
		// R2.1 + R2.2: width and goal together with file.
		{
			Name: "width_goal_with_file",
			Args: []string{"-w", "50", "-g", "40", longPara},
		},
		// R4.1: successful formatting exits 0 (implicit via default ExitCode=0).
		{
			Name:  "exit_zero_on_success",
			Stdin: []byte("A simple paragraph that formats successfully.\n"),
		},
		// R4.2: unrecognized long option produces error and exit 1.
		{
			Name:      "invalid_long_option",
			Args:      []string{"--nonexistent-flag"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4.2: invalid width value produces error and exit 1.
		{
			Name:      "invalid_width_value",
			Args:      []string{"--width=abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4.2: missing file with valid file produces partial output and exit 1.
		{
			Name:      "good_file_then_missing",
			Args:      []string{fileA, filepath.Join(tmpDir, "missing.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// TODO: R4.4 requires tests for -s split-only, -u uniform-spacing,
		// -p prefix, and -t tagged-paragraph. These flags are deferred to
		// rel99.0 per non_goals (R3 deferred). See GH-2938.
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// progNameRe matches path-prefixed program names (/path/to/fmt, /path/to/gfmt)
// and bare gfmt so they can be normalized to "fmt" for comparison.
var progNameRe = regexp.MustCompile(`[^\s']+/g?fmt|gfmt`)

// normalizeProgramName replaces the reference binary name (gfmt or
// /path/to/fmt) with fmt in output so that error messages can be compared.
func normalizeProgramName(data []byte) []byte {
	return progNameRe.ReplaceAll(data, []byte("fmt"))
}

// writeTestFile writes content to a file, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", path, err)
	}
}
