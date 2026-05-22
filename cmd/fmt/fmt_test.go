// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfmt")
	if err != nil {
		t.Skip("reference binary gfmt not found")
	}
	tests := []testutils.DiffTest{
		{Name: "short_line", Stdin: []byte("short line\n")},
		{Name: "empty_input", Stdin: []byte("")},
		{Name: "single_newline", Stdin: []byte("\n")},
		{Name: "default_75_wrap", Stdin: []byte(
			"This is a line that is definitely longer than seventy-five characters and should be wrapped by the formatter.\n",
		)},
		{Name: "blank_line_paragraph_sep", Stdin: []byte(
			"First paragraph line one.\nFirst paragraph line two.\n\nSecond paragraph line one.\n",
		)},
		{Name: "multiple_blank_lines", Stdin: []byte("Paragraph one.\n\n\nParagraph two.\n")},
		{Name: "indentation_preserved", Stdin: []byte(
			"  Indented first line of paragraph.\n  Indented second line of paragraph.\n",
		)},
		{Name: "stdin_dash", Args: []string{"-"}, Stdin: []byte("from stdin\n")},
		{Name: "multiple_spaces_preserved", Stdin: []byte("word1   word2    word3\n")},
		{Name: "sentence_end_spacing", Stdin: []byte("End of sentence.  Next sentence starts here.\n")},
		{Name: "trailing_blank_lines", Stdin: []byte("Some text.\n\n")},
		{Name: "leading_blank_lines", Stdin: []byte("\n\nSome text.\n")},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffSpaceCollapsing(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfmt")
	if err != nil {
		t.Skip("reference binary gfmt not found")
	}
	tests := []testutils.DiffTest{
		{Name: "collapse_multiple_spaces", Stdin: []byte(
			"word1    word2     word3      word4\n",
		)},
		{Name: "collapse_tabs", Stdin: []byte(
			"word1\tword2\t\tword3\n",
		)},
		{Name: "sentence_period_double_space", Stdin: []byte(
			"End of sentence. Next sentence starts here.\n",
		)},
		{Name: "sentence_exclamation_double_space", Stdin: []byte(
			"What a surprise! This continues the text.\n",
		)},
		{Name: "sentence_question_double_space", Stdin: []byte(
			"Is this a question? Yes it is a question.\n",
		)},
		{Name: "sentence_end_wrap", Args: []string{"-w", "30"}, Stdin: []byte(
			"End.  Start of next sentence here.\n",
		)},
		{Name: "join_paragraph_lines", Stdin: []byte(
			"First line of a paragraph.\nSecond line of a paragraph.\n",
		)},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffWordBoundaries(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfmt")
	if err != nil {
		t.Skip("reference binary gfmt not found")
	}
	tests := []testutils.DiffTest{
		{Name: "break_at_spaces", Args: []string{"-w", "20"}, Stdin: []byte(
			"one two three four five six\n",
		)},
		{Name: "no_mid_word_break", Args: []string{"-w", "10"}, Stdin: []byte(
			"verylongword short\n",
		)},
		{Name: "break_at_width", Args: []string{"-w", "25"}, Stdin: []byte(
			"hello world from the command line interface\n",
		)},
		{Name: "single_word_exceeds", Args: []string{"-w", "5"}, Stdin: []byte(
			"longword\n",
		)},
		{Name: "many_short_words", Args: []string{"-w", "20"}, Stdin: []byte(
			"a b c d e f g h i j k l m n o p q r s t\n",
		)},
		{Name: "mixed_word_lengths", Args: []string{"-w", "30"}, Stdin: []byte(
			"I am a short. Superlongwordthatexceedswidth tiny.\n",
		)},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffWidth(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfmt")
	if err != nil {
		t.Skip("reference binary gfmt not found")
	}
	tests := []testutils.DiffTest{
		{Name: "width_20", Args: []string{"-w", "20"}, Stdin: []byte(
			"This line should be wrapped at twenty characters.\n",
		)},
		{Name: "width_40", Args: []string{"-w", "40"}, Stdin: []byte(
			"This is a test of wrapping at forty characters wide.\n",
		)},
		{Name: "width_equals", Args: []string{"--width=30"}, Stdin: []byte(
			"Testing the long form width flag with equals sign.\n",
		)},
		{Name: "word_longer_than_width", Args: []string{"-w", "10"}, Stdin: []byte(
			"superlongwordthatcannotfit short\n",
		)},
		{Name: "goal_width", Args: []string{"-g", "30", "-w", "40"}, Stdin: []byte(
			"This is a test of the goal width feature of the fmt command.\n",
		)},
		{Name: "goal_long_form", Args: []string{"--goal=30", "--width=40"}, Stdin: []byte(
			"This is a test of the goal width feature of the fmt command.\n",
		)},
		{Name: "goal_default_93pct", Args: []string{"-w", "100"}, Stdin: []byte(
			"This is a line designed to test that the default goal width is ninety-three percent of the maximum width setting which is one hundred characters.\n",
		)},
		{Name: "goal_narrow", Args: []string{"-g", "20", "-w", "30"}, Stdin: []byte(
			"One two three four five six seven eight.\n",
		)},
		{Name: "goal_equals_width", Args: []string{"-g", "40", "-w", "40"}, Stdin: []byte(
			"When goal equals width the formatter fills lines to the maximum width.\n",
		)},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffModes(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfmt")
	if err != nil {
		t.Skip("reference binary gfmt not found")
	}
	tests := []testutils.DiffTest{
		{Name: "split_only", Args: []string{"-s"}, Stdin: []byte(
			"Short line.\nThis is a very long line that exceeds the default width of seventy-five characters and should be split.\n",
		)},
		{Name: "split_only_no_join", Args: []string{"-s", "-w", "40"}, Stdin: []byte(
			"Short.\nAlso short.\n",
		)},
		{Name: "uniform_spacing", Args: []string{"-u"}, Stdin: []byte(
			"Hello world.  This is a test.  More text here.\n",
		)},
		{Name: "uniform_multi_space", Args: []string{"-u"}, Stdin: []byte(
			"word1    word2     word3      word4\n",
		)},
		{Name: "uniform_tabs", Args: []string{"-u"}, Stdin: []byte(
			"word1\tword2\t\tword3\n",
		)},
		{Name: "uniform_sentence_triple", Args: []string{"-u"}, Stdin: []byte(
			"End.   Start.  More text here.\n",
		)},
		{Name: "uniform_sentence_single", Args: []string{"-u"}, Stdin: []byte(
			"End. Start.\n",
		)},
		{Name: "uniform_multi_paragraph", Args: []string{"-u"}, Stdin: []byte(
			"First  para   text.\n\nSecond   para   text.\n",
		)},
		{Name: "uniform_split", Args: []string{"-u", "-s"}, Stdin: []byte(
			"Short   line.\nThis is a very long   line    that   exceeds the default  width of   seventy-five characters and should be split.\n",
		)},
		{Name: "tagged_paragraph", Args: []string{"-t", "-w", "40"}, Stdin: []byte(
			"   First line tag.\nSecond line of the paragraph text.\nThird line.\n",
		)},
		{Name: "tagged_body_indent", Args: []string{"-t", "-w", "40"}, Stdin: []byte(
			"* Item header.\n  Body text that continues here and wraps around.\n",
		)},
		{Name: "tagged_multi_para", Args: []string{"-t", "-w", "40"}, Stdin: []byte(
			"  Tag one.\nBody of para one that wraps.\n\n  Tag two.\nBody of para two.\n",
		)},
		{Name: "tagged_long_form", Args: []string{"--tagged-paragraph", "-w", "40"}, Stdin: []byte(
			"   First line tag.\nSecond line body.\n",
		)},
		{Name: "split_long_mixed", Args: []string{"-s", "-w", "30"}, Stdin: []byte(
			"Short.\nThis is a longer line that should be split at thirty characters.\nAlso short.\n",
		)},
		{Name: "split_preserves_indent", Args: []string{"-s", "-w", "30"}, Stdin: []byte(
			"    This is an indented line that is too long to fit in thirty characters.\n",
		)},
		{Name: "split_no_join_paragraphs", Args: []string{"-s"}, Stdin: []byte(
			"First.\nSecond.\n\nThird.\nFourth.\n",
		)},
		{Name: "split_exact_width", Args: []string{"-s", "-w", "10"}, Stdin: []byte(
			"0123456789\n01234567890\n",
		)},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffFiles(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfmt")
	if err != nil {
		t.Skip("reference binary gfmt not found")
	}
	dir := t.TempDir()
	writeFixture(t, dir, "file1.txt", "Hello world from file one.\n")
	writeFixture(t, dir, "file2.txt", "Hello world from file two.\n")
	tests := []testutils.DiffTest{
		{Name: "single_file", Args: []string{"file1.txt"}, WorkDir: dir},
		{Name: "multi_file", Args: []string{"file1.txt", "file2.txt"}, WorkDir: dir},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffPrefix(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfmt")
	if err != nil {
		t.Skip("reference binary gfmt not found")
	}
	tests := []testutils.DiffTest{
		{Name: "prefix_basic", Args: []string{"-p", ">"}, Stdin: []byte(
			">This is a prefixed line that is long enough to need wrapping at seventy-five chars.\nNormal line.\n",
		)},
		{Name: "prefix_with_space", Args: []string{"-p", "> "}, Stdin: []byte(
			"> First prefixed line.\n> Second prefixed line.\nNot prefixed.\n",
		)},
		{Name: "prefix_indented", Args: []string{"-p", ">"}, Stdin: []byte(
			"  >This is a very long prefixed line that should definitely be wrapped because it exceeds the default width of seventy-five characters.\n",
		)},
		{Name: "prefix_blank_sep", Args: []string{"-p", ">"}, Stdin: []byte(
			">Line one of para one.\n>Line two of para one.\n>\n>Line one of para two.\nNot prefixed.\n",
		)},
		{Name: "prefix_long_form", Args: []string{"--prefix=>"}, Stdin: []byte(
			">Short prefixed.\nNormal.\n",
		)},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestMissingFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	cmd := exec.Command(goBin, "nonexistent.txt")
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit 1, got: %v", err)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("nonexistent.txt")) {
		t.Fatalf("stderr should mention nonexistent.txt, got %q", stderr.String())
	}
}

func TestDiffWidthZero(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfmt")
	if err != nil {
		t.Skip("reference binary gfmt not found")
	}
	tests := []testutils.DiffTest{
		{Name: "width_zero", Args: []string{"-w", "0"}, Stdin: []byte("test\n")},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestErrorCases(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	cases := []struct {
		name string
		args []string
	}{
		{"invalid_width_neg", []string{"-w", "-5"}},
		{"invalid_width_str", []string{"-w", "abc"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(goBin, tc.args...)
			cmd.Stdin = bytes.NewReader([]byte("test\n"))
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()
			exitErr, ok := err.(*exec.ExitError)
			if !ok || exitErr.ExitCode() != 1 {
				t.Fatalf("expected exit 1, got: %v", err)
			}
			if stderr.Len() == 0 {
				t.Fatal("expected error on stderr")
			}
		})
	}
}

func TestFileAndStdin(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfmt")
	if err != nil {
		t.Skip("reference binary gfmt not found")
	}
	dir := t.TempDir()
	writeFixture(t, dir, "input.txt", "Hello world.\n\nAnother paragraph here.\n")
	tests := []testutils.DiffTest{
		{Name: "file_with_paragraphs", Args: []string{"input.txt"}, WorkDir: dir},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
