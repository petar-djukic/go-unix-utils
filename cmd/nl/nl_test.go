// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd022-nl R1.1–R1.4, R2.1–R2.4.
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
