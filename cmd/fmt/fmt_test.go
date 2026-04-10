// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/fmt against gfmt (GNU coreutils).
// Implements srd070-fmt R13.3, R13.4.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeProgramName replaces gfmt: with fmt: in stderr so that
// diagnostic messages match between the reference and Go binaries.
func normalizeProgramName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gfmt:"), []byte("fmt:"))
}

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	if err != nil {
		t.Fatalf("writing test file %s: %v", name, err)
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfmt")
	if err != nil {
		t.Skipf("reference binary gfmt not in PATH: %v", err)
	}

	// R13.4: test inputs for various scenarios.
	longParagraph := "This is a long paragraph of text that exceeds the default " +
		"seventy-five character width and needs to be reformatted by the fmt " +
		"utility to fit within the specified line width properly."

	indentedText := "  First line with indent.\n" +
		"  Second line continues the paragraph with more words to fill.\n" +
		"  Third line also indented.\n"

	taggedText := "    First line has extra indent.\n" +
		"  Second line has less indent and continues with more text to fill.\n" +
		"  Third line also continues.\n"

	prefixedText := "> This is a prefixed line that should be reformatted.\n" +
		"> Another prefixed line that continues the paragraph.\n" +
		"This line has no prefix and should not be reformatted.\n"

	multiSpaceText := "Word   with    multiple   spaces   between   them   here.\n"

	splitText := "short\n" +
		"a very long line that definitely exceeds the default width of " +
		"seventy-five characters and should be split into multiple lines by " +
		"the split-only mode\n"

	paraText := "first paragraph line one.\n" +
		"first paragraph line two.\n" +
		"\n" +
		"second paragraph line one.\n" +
		"second paragraph line two.\n"

	// Create a temp dir for file-based tests.
	fileDir := t.TempDir()
	writeTestFile(t, fileDir, "input.txt", longParagraph+"\n")
	writeTestFile(t, fileDir, "para.txt", paraText)
	writeTestFile(t, fileDir, "second.txt", "Another file content here.\n")

	tests := []testutils.DiffTest{
		// R1.1, R4.1: default 75-char formatting from stdin
		{
			Name:  "default_width_stdin",
			Stdin: []byte(longParagraph + "\n"),
		},
		// R1.1: short line unchanged
		{
			Name:  "short_line_unchanged",
			Stdin: []byte("short line\n"),
		},
		// R5.1: -w custom width
		{
			Name:  "custom_width_w30",
			Args:  []string{"-w", "30"},
			Stdin: []byte("a line of text that exceeds thirty characters\n"),
		},
		// R5.1: --width=N long form
		{
			Name:  "custom_width_long_form",
			Args:  []string{"--width=40"},
			Stdin: []byte("a line of text that exceeds forty characters in width\n"),
		},
		// R6.1: -g goal width
		{
			Name:  "goal_width",
			Args:  []string{"-w", "60", "-g", "40"},
			Stdin: []byte(longParagraph + "\n"),
		},
		// R6.1: --goal=N long form
		{
			Name:  "goal_width_long_form",
			Args:  []string{"--width=60", "--goal=40"},
			Stdin: []byte(longParagraph + "\n"),
		},
		// R9.1: -s split-only mode
		{
			Name:  "split_only",
			Args:  []string{"-s"},
			Stdin: []byte(splitText),
		},
		// R9.1: --split-only long form
		{
			Name:  "split_only_long_form",
			Args:  []string{"--split-only"},
			Stdin: []byte(splitText),
		},
		// R10.1: -u uniform spacing
		{
			Name:  "uniform_spacing",
			Args:  []string{"-u"},
			Stdin: []byte(multiSpaceText),
		},
		// R10.1: --uniform-spacing long form
		{
			Name:  "uniform_spacing_long_form",
			Args:  []string{"--uniform-spacing"},
			Stdin: []byte(multiSpaceText),
		},
		// R11.1: -p prefix mode
		{
			Name:  "prefix_mode",
			Args:  []string{"-p", ">"},
			Stdin: []byte(prefixedText),
		},
		// R11.1: --prefix=PREFIX long form
		{
			Name:  "prefix_mode_long_form",
			Args:  []string{"--prefix=>"},
			Stdin: []byte(prefixedText),
		},
		// R12.1: -t tagged paragraph
		{
			Name:  "tagged_paragraph",
			Args:  []string{"-t"},
			Stdin: []byte(taggedText),
		},
		// R12.1: --tagged-paragraph long form
		{
			Name:  "tagged_paragraph_long_form",
			Args:  []string{"--tagged-paragraph"},
			Stdin: []byte(taggedText),
		},
		// R2.1: blank line paragraph boundaries
		{
			Name:  "paragraph_boundaries",
			Args:  []string{"-w", "40"},
			Stdin: []byte(paraText),
		},
		// R3.1: indentation preservation
		{
			Name:  "indentation_preservation",
			Stdin: []byte(indentedText),
		},
		// R4.1: file input
		{
			Name:    "file_input",
			Args:    []string{"input.txt"},
			WorkDir: fileDir,
		},
		// R4.1: multi-file input
		{
			Name:    "multi_file_input",
			Args:    []string{"input.txt", "second.txt"},
			WorkDir: fileDir,
		},
		// R4.1: stdin via "-"
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("hello world\n"),
		},
		// R8.1: sentence-ending punctuation gets two spaces
		{
			Name:  "sentence_spacing",
			Args:  []string{"-w", "60"},
			Stdin: []byte("First sentence. Second sentence. Third sentence.\n"),
		},
		// R7.1: word boundaries preserved
		{
			Name:  "word_boundary_wrap",
			Args:  []string{"-w", "20"},
			Stdin: []byte("one two three four five six seven eight\n"),
		},
		// R5.1 + R9.1: -s with custom width
		{
			Name:  "split_only_custom_width",
			Args:  []string{"-s", "-w", "30"},
			Stdin: []byte("short\na longer line that exceeds thirty characters\n"),
		},
		// R13.1: exit 0 on success (empty input)
		{
			Name:  "exit_0_empty_input",
			Stdin: []byte(""),
		},
		// R13.2: missing file error
		{
			Name:      "missing_file_error",
			Args:      []string{"nonexistent_file.txt"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R13.2: invalid width value
		{
			Name:      "invalid_width",
			Args:      []string{"-w", "abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R1.1: multiple blank lines between paragraphs
		{
			Name:  "multiple_blank_lines",
			Stdin: []byte("first paragraph\n\n\nsecond paragraph\n"),
		},
		// R1.1: single word exceeding width
		{
			Name:  "long_word_exceeds_width",
			Args:  []string{"-w", "10"},
			Stdin: []byte("averylongwordthatexceedswidth\n"),
		},
		// R12.1 + R5.1: tagged paragraph with custom width
		{
			Name:  "tagged_paragraph_custom_width",
			Args:  []string{"-t", "-w", "40"},
			Stdin: []byte(taggedText),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
