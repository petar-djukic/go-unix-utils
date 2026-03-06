// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/nl (prd022-nl R4).
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests comparing Go nl against gnl.
// R4: byte-for-byte comparison via RunDiffTests.
// D3: LC_ALL=C set via DiffTest.Env.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// D2: Graceful skip if gnl is not in PATH.
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skipf("reference binary gnl not in PATH: %v", err)
	}

	// Create temp files for file-based tests.
	dir := t.TempDir()
	file1 := writeTestFile(t, dir, "f1.txt", "alpha\nbeta\ngamma\n")
	file2 := writeTestFile(t, dir, "f2.txt", "x\ny\n")
	fileSections := writeTestFile(t, dir, "sections.txt",
		"\\:\\:\\:\nheader line\n\\:\\:\nbody line\n\\:\nfooter line\n")

	tests := []testutils.DiffTest{
		// R1.1, R1.2: Default numbering — non-empty body lines numbered, empty lines pass through.
		{
			Name:  "nl_default_body_numbering",
			Stdin: []byte("first\n\nsecond\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -b a numbers all lines including empty lines.
		{
			Name:  "nl_number_all_lines",
			Args:  []string{"-b", "a"},
			Stdin: []byte("x\n\ny\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1, R3.2, R3.3: Format, width, and separator.
		{
			Name:  "nl_format_ln_width_3_separator",
			Args:  []string{"-n", "ln", "-w", "3", "-s", ": "},
			Stdin: []byte("a\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.4: Start and increment.
		{
			Name:  "nl_start_and_increment",
			Args:  []string{"-v", "10", "-i", "5"},
			Stdin: []byte("p\nq\nr\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.1, R4.2: Section delimiters consumed; header numbered with -h a.
		{
			Name:  "nl_section_delimiters",
			Args:  []string{"-h", "a"},
			Stdin: []byte("\\:\\:\\:\nheader line\n\\:\\:\nbody line\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: Read from file argument.
		{
			Name: "nl_file_arg",
			Args: []string{file1},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: Multiple files — numbering continuous across files.
		{
			Name: "nl_multifile",
			Args: []string{file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: Stdin via "-" argument.
		{
			Name:  "nl_stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("one\ntwo\nthree\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Empty input produces no output.
		{
			Name:  "nl_empty_input",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: Header style -h a with file containing section delimiters.
		{
			Name: "nl_section_file",
			Args: []string{"-h", "a", fileSections},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: Footer style -f a numbers footer lines.
		{
			Name:  "nl_footer_style",
			Args:  []string{"-f", "a"},
			Stdin: []byte("\\:\\:\\:\nheader\n\\:\\:\nbody\n\\:\nfooter\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: Style n — no numbering for body.
		{
			Name:  "nl_body_style_none",
			Args:  []string{"-b", "n"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: Right-justified with leading zeros.
		{
			Name:  "nl_format_rz",
			Args:  []string{"-n", "rz", "-w", "4"},
			Stdin: []byte("x\ny\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.3: -p suppresses counter reset.
		{
			Name:  "nl_no_reset",
			Args:  []string{"-p", "-h", "a"},
			Stdin: []byte("\\:\\:\\:\nheader\n\\:\\:\nbody\n\\:\\:\\:\nheader2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -b p — number lines matching regex.
		{
			Name:  "nl_regex_style",
			Args:  []string{"-b", "p^[a-z]"},
			Stdin: []byte("abc\n123\ndef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.3: Custom separator string.
		{
			Name:  "nl_custom_separator",
			Args:  []string{"-s", "->"},
			Stdin: []byte("hello\nworld\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Single line input.
		{
			Name:  "nl_single_line",
			Stdin: []byte("only\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.4: -l 2 join blank lines.
		{
			Name:  "nl_join_blank",
			Args:  []string{"-b", "a", "-l", "2"},
			Stdin: []byte("a\n\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: Left-justified default width.
		{
			Name:  "nl_format_ln_default_width",
			Args:  []string{"-n", "ln"},
			Stdin: []byte("a\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Multiple empty lines without -b a.
		{
			Name:  "nl_multiple_empty_lines",
			Stdin: []byte("a\n\n\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile creates a file in dir with the given content and returns its path.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file %s: %v", name, err)
	}
	return path
}
