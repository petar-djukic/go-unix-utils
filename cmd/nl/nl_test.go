// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd022-nl R1.1–R1.4, R2.1–R2.4, R3.1–R3.4.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrErrorNormalizer strips stderr error lines that start with the
// program name (nl or gnl). The error message format and binary name
// differ between GNU and Go, but the exit code and stdout are what matter.
var stderrErrorNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	re := regexp.MustCompile(`(?m)^g?nl: .*\n?`)
	return re.ReplaceAll(data, nil)
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skipf("reference binary gnl not in PATH: %v", err)
	}

	// R1.4: multi-file continuous numbering
	multiFile1 := createTempFile(t, "a\nb\n")
	multiFile2 := createTempFile(t, "c\nd\n")
	// Error test: valid file for mixed scenarios
	validFile := createTempFile(t, "hello\nworld\n")

	tests := []testutils.DiffTest{
		// === R1: Default line numbering ===
		{
			// R1.1, R1.2: non-empty lines numbered, empty lines passed through
			Name:  "default_mixed_empty",
			Stdin: []byte("a\n\nb\n"),
		},
		{
			// R1.1: all non-empty lines numbered from stdin
			Name:  "default_nonempty_only",
			Stdin: []byte("first\nsecond\nthird\n"),
		},
		{
			// R1.2: all empty lines
			Name:  "all_empty_lines",
			Stdin: []byte("\n\n\n"),
		},
		{
			// R1.1: single non-empty line
			Name:  "single_nonempty_line",
			Stdin: []byte("only\n"),
		},
		{
			// R1.3: empty input from stdin
			Name:  "empty_input",
			Stdin: []byte{},
		},
		{
			// R1.3: stdin via explicit dash argument
			Name:  "stdin_dash_arg",
			Args:  []string{"-"},
			Stdin: []byte("foo\nbar\n"),
		},
		{
			// R1.1: line with spaces is non-empty, should be numbered
			Name:  "line_with_spaces",
			Stdin: []byte("  \nhello\n"),
		},
		{
			// R1.4: continuous numbering across two files
			Name: "multi_file_continuous",
			Args: []string{multiFile1, multiFile2},
		},
		{
			// R1.1, R1.2: multiple empty lines between content
			Name:  "multiple_empty_between",
			Stdin: []byte("a\n\n\nb\n"),
		},
		{
			// R1.1: no trailing newline on last line
			Name:  "no_trailing_newline",
			Stdin: []byte("a\nb"),
		},
		{
			// Error: nonexistent file exits 1
			Name:      "nonexistent_file",
			Args:      []string{"/nonexistent/path/file.txt"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrErrorNormalizer},
		},
		{
			// Error: valid file after nonexistent still produces output
			Name:      "nonexistent_then_valid",
			Args:      []string{"/nonexistent/path/file.txt", validFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrErrorNormalizer},
		},

		// === R2.1: -b STYLE body numbering style ===
		{
			// R2.1: -b a numbers all lines including empty
			Name:  "body_style_a",
			Args:  []string{"-b", "a"},
			Stdin: []byte("a\n\nb\n"),
		},
		{
			// R2.1: -b t is explicit default (number non-empty)
			Name:  "body_style_t_explicit",
			Args:  []string{"-b", "t"},
			Stdin: []byte("a\n\nb\n"),
		},
		{
			// R2.1, R2.4: -b n numbers no lines
			Name:  "body_style_n",
			Args:  []string{"-b", "n"},
			Stdin: []byte("a\nb\n"),
		},
		{
			// R2.1: -b pRE numbers lines matching regex
			Name:  "body_style_p_regex",
			Args:  []string{"-b", "p^[ab]"},
			Stdin: []byte("abc\ndef\naxy\n"),
		},
		{
			// R2.1: combined -ba form
			Name:  "body_style_combined_ba",
			Args:  []string{"-ba"},
			Stdin: []byte("x\n\ny\n"),
		},
		{
			// R2.1: combined -bn form
			Name:  "body_style_combined_bn",
			Args:  []string{"-bn"},
			Stdin: []byte("x\ny\n"),
		},

		// === R2.2: -h STYLE header numbering style ===
		{
			// R2.2: -h a numbers header section lines
			Name:  "header_style_a",
			Args:  []string{"-h", "a"},
			Stdin: []byte("\\:\\:\\:\nheader1\nheader2\n\\:\\:\nbody1\n"),
		},
		{
			// R2.2: -h t numbers non-empty header lines
			Name:  "header_style_t",
			Args:  []string{"-h", "t"},
			Stdin: []byte("\\:\\:\\:\nheader\n\n\\:\\:\nbody\n"),
		},

		// === R2.3: -f STYLE footer numbering style ===
		{
			// R2.3: -f a numbers footer section lines
			Name:  "footer_style_a",
			Args:  []string{"-f", "a"},
			Stdin: []byte("\\:\\:\\:\nheader\n\\:\\:\nbody\n\\:\nfooter1\nfooter2\n"),
		},
		{
			// R2.3: -f t numbers non-empty footer lines
			Name:  "footer_style_t",
			Args:  []string{"-f", "t"},
			Stdin: []byte("\\:\\:\\:\nheader\n\\:\\:\nbody\n\\:\nfooter\n\n"),
		},

		// === R2.4: style n produces no number and no separator ===
		{
			// R2.4: -b n with multiple lines, no numbers
			Name:  "style_n_passthrough",
			Args:  []string{"-b", "n"},
			Stdin: []byte("hello\n\nworld\n"),
		},

		// === Combined section styles ===
		{
			// R2.1-R2.3: all sections with different styles
			Name:  "all_sections_styled",
			Args:  []string{"-h", "a", "-b", "a", "-f", "a"},
			Stdin: []byte("\\:\\:\\:\nhdr\n\\:\\:\nbod\n\\:\nftr\n"),
		},
		{
			// R2.1-R2.3: body a with default header/footer n
			Name:  "body_a_default_hf",
			Args:  []string{"-b", "a"},
			Stdin: []byte("\\:\\:\\:\nhdr\n\\:\\:\nbod\n\\:\nftr\n"),
		},

		// === R3.1: -n FORMAT line number format ===
		{
			// R3.1: -n ln left-justified
			Name:  "format_ln",
			Args:  []string{"-n", "ln"},
			Stdin: []byte("a\nb\n"),
		},
		{
			// R3.1: -n rn right-justified (explicit default)
			Name:  "format_rn_explicit",
			Args:  []string{"-n", "rn"},
			Stdin: []byte("a\nb\n"),
		},
		{
			// R3.1: -n rz right-justified with leading zeros
			Name:  "format_rz",
			Args:  []string{"-n", "rz"},
			Stdin: []byte("a\nb\n"),
		},
		{
			// R3.1: -n ln with -b a to include empty lines
			Name:  "format_ln_body_all",
			Args:  []string{"-n", "ln", "-b", "a"},
			Stdin: []byte("x\n\ny\n"),
		},
		{
			// R3.1: combined -nln form
			Name:  "format_ln_combined",
			Args:  []string{"-nln"},
			Stdin: []byte("a\nb\n"),
		},
		{
			// R3.1: combined -nrz form
			Name:  "format_rz_combined",
			Args:  []string{"-nrz"},
			Stdin: []byte("a\nb\n"),
		},

		// === R3.2: -w N field width ===
		{
			// R3.2: -w 3 narrow width
			Name:  "width_3",
			Args:  []string{"-w", "3"},
			Stdin: []byte("a\nb\n"),
		},
		{
			// R3.2: -w 10 wide width
			Name:  "width_10",
			Args:  []string{"-w", "10"},
			Stdin: []byte("a\nb\n"),
		},
		{
			// R3.2: -w 1 minimum width
			Name:  "width_1",
			Args:  []string{"-w", "1"},
			Stdin: []byte("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"),
		},
		{
			// R3.2: combined -w3 form
			Name:  "width_combined",
			Args:  []string{"-w3"},
			Stdin: []byte("a\nb\n"),
		},
		{
			// R3.2: -w with -n rz
			Name:  "width_with_rz",
			Args:  []string{"-w", "4", "-n", "rz"},
			Stdin: []byte("a\nb\n"),
		},

		// === R3.3: -s SEP separator string ===
		{
			// R3.3: -s with colon-space separator
			Name:  "sep_colon_space",
			Args:  []string{"-s", ": "},
			Stdin: []byte("a\nb\n"),
		},
		{
			// R3.3: -s with empty separator
			Name:  "sep_empty",
			Args:  []string{"-s", ""},
			Stdin: []byte("a\nb\n"),
		},
		{
			// R3.3: -s with multi-char separator
			Name:  "sep_multi_char",
			Args:  []string{"-s", " | "},
			Stdin: []byte("a\nb\n"),
		},
		{
			// R3.3: combined form -s with -n ln -w 3
			Name:  "sep_with_format_width",
			Args:  []string{"-n", "ln", "-w", "3", "-s", ": "},
			Stdin: []byte("a\nb\n"),
		},

		// === R3.4: -v N start value and -i N increment ===
		{
			// R3.4: -v 10 start at 10
			Name:  "start_value_10",
			Args:  []string{"-v", "10"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R3.4: -i 5 increment by 5
			Name:  "increment_5",
			Args:  []string{"-i", "5"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R3.4: -v 10 -i 5 combined
			Name:  "start_10_increment_5",
			Args:  []string{"-v", "10", "-i", "5"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R3.4: -v 0 start at zero
			Name:  "start_value_0",
			Args:  []string{"-v", "0"},
			Stdin: []byte("a\nb\n"),
		},
		{
			// R3.4: -i 2 with empty lines (only non-empty incremented)
			Name:  "increment_2_with_empty",
			Args:  []string{"-i", "2"},
			Stdin: []byte("a\n\nb\n"),
		},
		{
			// R3.4: combined -v10 and -i5 forms
			Name:  "combined_v_i",
			Args:  []string{"-v10", "-i5"},
			Stdin: []byte("a\nb\n"),
		},

		// === R3 combined: multiple R3 flags together ===
		{
			// R3.1-R3.4: all format options combined
			Name:  "all_format_options",
			Args:  []string{"-b", "a", "-n", "ln", "-w", "3", "-s", ": ", "-v", "10", "-i", "5"},
			Stdin: []byte("a\nb\n"),
		},
		{
			// R3.1-R3.4: rz format with custom width, start, increment
			Name:  "rz_custom_all",
			Args:  []string{"-n", "rz", "-w", "4", "-v", "100", "-i", "10"},
			Stdin: []byte("a\nb\nc\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestWriteError verifies exit 1 when a write error occurs on stdout.
func TestWriteError(t *testing.T) {
	t.Parallel()

	w := &failWriter{}
	var stderr bytes.Buffer
	code := run(nil, bytes.NewReader([]byte("a\nb\n")), w, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1 on write error, got %d", code)
	}
}

// failWriter is an io.Writer that always returns an error.
type failWriter struct{}

func (f *failWriter) Write([]byte) (int, error) {
	return 0, os.ErrClosed
}

// createTempFile writes content to a temporary file and returns its path.
func createTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	return path
}
