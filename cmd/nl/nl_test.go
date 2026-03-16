// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/nl against gnl (GNU coreutils).
// Implements prd022-nl R1.1-R1.4, R2.1-R2.4, R4.1-R4.2 test coverage.
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
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skipf("reference binary gnl not in PATH: %v", err)
	}

	// Create test fixtures in a temp directory.
	tmpDir := t.TempDir()
	writeTestFile(t, tmpDir, "three-lines.txt", "first\n\nsecond\n")
	writeTestFile(t, tmpDir, "abc.txt", "a\nb\nc\n")
	writeTestFile(t, tmpDir, "def.txt", "d\ne\nf\n")
	writeTestFile(t, tmpDir, "empty.txt", "")
	writeTestFile(t, tmpDir, "no-trailing-newline.txt", "hello\nworld")
	writeTestFile(t, tmpDir, "all-empty-lines.txt", "\n\n\n")
	writeTestFile(t, tmpDir, "mixed.txt", "alpha\n\nbeta\n\ngamma\n")

	tests := []testutils.DiffTest{
		// R1.1: default mode numbers non-empty body lines with width 6 + tab.
		{
			Name:  "R1.1_default_body_numbering",
			Stdin: []byte("first\n\nsecond\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: all non-empty lines numbered sequentially.
		{
			Name:  "R1.1_sequential_numbering",
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: empty lines pass through unnumbered.
		{
			Name:  "R1.2_empty_lines_unnumbered",
			Stdin: []byte("x\n\n\ny\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: input of only empty lines — none numbered.
		{
			Name:  "R1.2_all_empty_lines",
			Stdin: []byte("\n\n\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: read from stdin when no arguments.
		{
			Name:  "R1.3_stdin_no_args",
			Stdin: []byte("hello\nworld\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: "-" means stdin.
		{
			Name:  "R1.3_dash_means_stdin",
			Args:  []string{"-"},
			Stdin: []byte("foo\nbar\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: read from a named file.
		{
			Name:    "R1.2_named_file",
			Args:    []string{filepath.Join(tmpDir, "three-lines.txt")},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},
		// R1.4: continuous numbering across multiple files.
		{
			Name: "R1.4_continuous_numbering_multifile",
			Args: []string{
				filepath.Join(tmpDir, "abc.txt"),
				filepath.Join(tmpDir, "def.txt"),
			},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},
		// R1.4: nonexistent file exits >0 with error on stderr.
		{
			Name:      "R1.4_nonexistent_file",
			Args:      []string{filepath.Join(tmpDir, "does-not-exist.txt")},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R1.4: nonexistent mixed with existing files — exit 1, still
		// processes existing files with continuous numbering.
		{
			Name: "R1.4_nonexistent_mixed",
			Args: []string{
				filepath.Join(tmpDir, "abc.txt"),
				filepath.Join(tmpDir, "does-not-exist.txt"),
				filepath.Join(tmpDir, "def.txt"),
			},
			WorkDir:   tmpDir,
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// Edge: empty input from stdin.
		{
			Name:  "empty_stdin",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// Edge: empty file.
		{
			Name:    "empty_file",
			Args:    []string{filepath.Join(tmpDir, "empty.txt")},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},
		// Edge: single line.
		{
			Name:  "single_line",
			Stdin: []byte("only\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Edge: file without trailing newline.
		{
			Name:    "no_trailing_newline",
			Args:    []string{filepath.Join(tmpDir, "no-trailing-newline.txt")},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},
		// Edge: multiple empty lines interspersed with content.
		{
			Name:    "mixed_empty_and_content",
			Args:    []string{filepath.Join(tmpDir, "mixed.txt")},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},
		// Edge: single empty line.
		{
			Name:  "single_empty_line",
			Stdin: []byte("\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffR2(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skipf("reference binary gnl not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R2.1: -b a numbers all lines including empty ones.
		{
			Name:  "R2.1_body_style_a_all_lines",
			Args:  []string{"-b", "a"},
			Stdin: []byte("first\n\nsecond\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -ba concatenated form.
		{
			Name:  "R2.1_body_style_a_concatenated",
			Args:  []string{"-ba"},
			Stdin: []byte("x\n\ny\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -b t numbers non-empty lines only (explicit default).
		{
			Name:  "R2.1_body_style_t_explicit",
			Args:  []string{"-b", "t"},
			Stdin: []byte("a\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -b n numbers no lines.
		{
			Name:  "R2.1_body_style_n_none",
			Args:  []string{"-b", "n"},
			Stdin: []byte("hello\nworld\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -b pRE numbers lines matching regex.
		{
			Name:  "R2.1_body_style_p_regex",
			Args:  []string{"-bp^[aeiou]"},
			Stdin: []byte("apple\nbanana\norange\ngrape\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -b p with separate arg for regex.
		{
			Name:  "R2.1_body_style_p_regex_separate",
			Args:  []string{"-b", "p^[0-9]"},
			Stdin: []byte("123\nabc\n456\ndef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: style n outputs lines with no number and no separator.
		{
			Name:  "R2.4_style_n_no_number_no_separator",
			Args:  []string{"-bn"},
			Stdin: []byte("alpha\nbeta\n\ngamma\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -b a with all empty lines — all get numbered.
		{
			Name:  "R2.1_body_style_a_all_empty",
			Args:  []string{"-ba"},
			Stdin: []byte("\n\n\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -b a with mixed content.
		{
			Name:  "R2.1_body_style_a_mixed",
			Args:  []string{"-ba"},
			Stdin: []byte("one\n\ntwo\n\n\nthree\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -bp with dot-star regex matches all non-empty.
		{
			Name:  "R2.1_body_style_p_dotstar",
			Args:  []string{"-bp."},
			Stdin: []byte("foo\n\nbar\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: -bn with empty lines — all lines pass through unnumbered.
		{
			Name:  "R2.4_style_n_with_empty",
			Args:  []string{"-bn"},
			Stdin: []byte("x\n\ny\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: -h flag accepted (header style, no effect without section delimiters).
		{
			Name:  "R2.2_header_style_flag_accepted",
			Args:  []string{"-ha"},
			Stdin: []byte("line1\nline2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: -f flag accepted (footer style, no effect without section delimiters).
		{
			Name:  "R2.3_footer_style_flag_accepted",
			Args:  []string{"-fn"},
			Stdin: []byte("line1\nline2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1-R2.3: combined body and header style flags.
		{
			Name:  "R2.1_R2.2_combined_b_h_flags",
			Args:  []string{"-ba", "-ha"},
			Stdin: []byte("data\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffR4 covers section delimiter handling (prd022-nl R4.1-R4.2).
func TestDiffR4(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skipf("reference binary gnl not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.1: header delimiter produces empty line and starts header section.
		{
			Name:  "R4.1_header_delimiter_to_empty_line",
			Stdin: []byte("\\:\\:\\:\nheader line\n\\:\\:\nbody line\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.1: footer delimiter switches to footer section.
		{
			Name:  "R4.1_footer_delimiter",
			Stdin: []byte("body\n\\:\nfooter\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.2: header delimiter resets line counter.
		{
			Name:  "R4.2_header_resets_counter",
			Stdin: []byte("a\nb\n\\:\\:\\:\nheader\n\\:\\:\nc\nd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.1: body and footer delimiters also reset counter (GNU behavior).
		{
			Name:  "R4.1_body_footer_reset",
			Stdin: []byte("a\n\\:\\:\nb\n\\:\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: -h a numbers header lines with section delimiters.
		{
			Name:  "R2.2_header_style_a_with_delimiters",
			Args:  []string{"-ha"},
			Stdin: []byte("\\:\\:\\:\nheader1\nheader2\n\\:\\:\nbody1\nbody2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: -f a numbers footer lines with section delimiters.
		{
			Name:  "R2.3_footer_style_a_with_delimiters",
			Args:  []string{"-fa"},
			Stdin: []byte("body1\n\\:\nfooter1\nfooter2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2+R2.3: default header and footer styles are n (no numbering).
		{
			Name:  "R2.2_R2.3_default_header_footer_not_numbered",
			Stdin: []byte("\\:\\:\\:\nheader\n\\:\\:\nbody\n\\:\nfooter\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.2: multiple logical pages reset counter on each header.
		{
			Name:  "R4.2_multiple_pages_reset",
			Stdin: []byte("a\nb\n\\:\\:\\:\n\\:\\:\nc\nd\n\\:\\:\\:\n\\:\\:\ne\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// All three sections with all-number styles.
		{
			Name:  "all_sections_style_a",
			Args:  []string{"-ha", "-ba", "-fa"},
			Stdin: []byte("\\:\\:\\:\nh1\n\\:\\:\nb1\n\\:\nf1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Edge: delimiter line that is not exact (has trailing content).
		{
			Name:  "delimiter_with_trailing_content_not_delimiter",
			Stdin: []byte("\\:\\:\\:extra\nline\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Edge: consecutive delimiters.
		{
			Name:  "consecutive_delimiters",
			Stdin: []byte("\\:\\:\\:\n\\:\\:\n\\:\n"),
			Env:   []string{"LC_ALL=C"},
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

// normalizeProgramName normalizes error messages for differential comparison.
// GNU nl reports errors as "gnl: ..." while our binary uses "nl: ...". This
// normalizer replaces the program name and lowercases to eliminate both differences.
func normalizeProgramName(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gnl: "), []byte("nl: "))
	return bytes.ToLower(b)
}
