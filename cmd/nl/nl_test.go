// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// writeTestFile is a helper that writes content to path.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestDiff runs differential tests against the GNU reference binary (gnl).
// Covers prd022-nl R1.1 (default numbering format), R1.2 (empty lines),
// R1.3 (stdin input).
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skipf("reference binary gnl not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R1.1, R1.2: non-empty lines numbered, empty line passed through.
			Name:  "default_body_numbering",
			Stdin: []byte("first\n\nsecond\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.2: all empty lines produce bare newlines.
			Name:  "all_empty_lines",
			Stdin: []byte("\n\n\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: single non-empty line numbered at 1.
			Name:  "single_line",
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.2: consecutive empty lines between content.
			Name:  "consecutive_empty_between",
			Stdin: []byte("a\n\n\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: multiple non-empty lines numbered sequentially.
			Name:  "multiple_lines",
			Stdin: []byte("alpha\nbeta\ngamma\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.3: stdin with no trailing newline.
			Name:  "no_trailing_newline",
			Stdin: []byte("line"),
			Env:   []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffStyleFlags runs differential tests for numbering style flags.
// Covers prd022-nl R2.1 (-b STYLE), R2.2 (-h STYLE), R2.3 (-f STYLE),
// R2.4 (style n produces no number and no separator).
func TestDiffStyleFlags(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skipf("reference binary gnl not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R2.1: -b a numbers all lines including empty.
			Name:  "body_style_a",
			Args:  []string{"-b", "a"},
			Stdin: []byte("x\n\ny\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.1: -b t numbers only non-empty lines (default).
			Name:  "body_style_t",
			Args:  []string{"-b", "t"},
			Stdin: []byte("x\n\ny\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.1, R2.4: -b n numbers no lines; content passes through.
			Name:  "body_style_n",
			Args:  []string{"-b", "n"},
			Stdin: []byte("hello\nworld\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.1: -b pRE numbers lines matching regex.
			Name:  "body_style_regex",
			Args:  []string{"-b", "p^[A-Z]"},
			Stdin: []byte("Alpha\nbeta\nGamma\ndelta\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.2: -h a numbers header section lines.
			Name:  "header_style_a",
			Args:  []string{"-h", "a"},
			Stdin: []byte("\\:\\:\\:\nheader line\n\\:\\:\nbody line\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.2, R2.4: -h n (default) does not number header lines.
			Name:  "header_style_n_default",
			Stdin: []byte("\\:\\:\\:\nheader line\n\\:\\:\nbody line\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.3: -f a numbers footer section lines.
			Name:  "footer_style_a",
			Args:  []string{"-f", "a"},
			Stdin: []byte("\\:\\:\\:\nheader\n\\:\\:\nbody\n\\:\nfooter\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.3, R2.4: -f n (default) does not number footer lines.
			Name:  "footer_style_n_default",
			Stdin: []byte("\\:\\:\\:\nheader\n\\:\\:\nbody\n\\:\nfooter\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.4: all sections set to n; no lines numbered.
			Name:  "all_sections_style_n",
			Args:  []string{"-b", "n", "-h", "n", "-f", "n"},
			Stdin: []byte("\\:\\:\\:\nhdr\n\\:\\:\nbdy\n\\:\nftr\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.1: -b a with empty lines among content.
			Name:  "body_all_with_empty",
			Args:  []string{"-b", "a"},
			Stdin: []byte("\n\nfoo\n\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.1: regex style with no matches.
			Name:  "body_regex_no_match",
			Args:  []string{"-b", "p^NOMATCH"},
			Stdin: []byte("alpha\nbeta\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFileInput runs differential tests for named file input and
// continuous numbering across multiple files.
// Covers prd022-nl R1.3 (named file reading), R1.4 (continuous numbering).
func TestDiffFileInput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skipf("reference binary gnl not in PATH: %v", err)
	}

	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	writeTestFile(t, f1, "alpha\nbeta\n")
	writeTestFile(t, f2, "gamma\ndelta\n")

	tests := []testutils.DiffTest{
		{
			// R1.3: read from a single named file.
			Name: "single_file_input",
			Args: []string{f1},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R1.4: numbering continues across files without reset.
			Name: "continuous_across_files",
			Args: []string{f1, f2},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
