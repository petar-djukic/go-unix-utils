// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/fold against the GNU reference binary (gfold).
// Implements prd023-fold R1.1-R1.4, R2.1-R2.3, R3.1-R3.4 test coverage.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfold")
	if err != nil {
		t.Skipf("reference binary gfold not in PATH: %v", err)
	}

	// Create a temp file for file-input tests.
	tmpDir := t.TempDir()
	longFile := filepath.Join(tmpDir, "long.txt")
	longLine := strings.Repeat("abcdefghij", 10) + "\n" // 100 chars
	if err := os.WriteFile(longFile, []byte(longLine), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	shortFile := filepath.Join(tmpDir, "short.txt")
	if err := os.WriteFile(shortFile, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R1.1: read from stdin and wrap at default 80 columns.
			Name:  "stdin_default_width",
			Stdin: []byte(strings.Repeat("x", 100) + "\n"),
		},
		{
			// R1.2: lines shorter than width pass through unchanged.
			Name:  "short_line_unchanged",
			Stdin: []byte("short line\n"),
		},
		{
			// R1.2: line exactly 80 chars passes through unchanged.
			Name:  "exact_80_chars",
			Stdin: []byte(strings.Repeat("a", 80) + "\n"),
		},
		{
			// R1.3: line longer than 80 is split.
			Name:  "wrap_at_80",
			Stdin: []byte(strings.Repeat("b", 160) + "\n"),
		},
		{
			// R1.3: wrapping applied repeatedly.
			Name:  "triple_wrap",
			Stdin: []byte(strings.Repeat("c", 200) + "\n"),
		},
		{
			// R1.4: no trailing newline preserved.
			Name:  "no_trailing_newline",
			Stdin: []byte(strings.Repeat("d", 100)),
		},
		{
			// R1.4: trailing newline preserved.
			Name:  "trailing_newline_preserved",
			Stdin: []byte(strings.Repeat("e", 100) + "\n"),
		},
		{
			// R1.1: wrap with -w flag.
			Name:  "custom_width",
			Args:  []string{"-w", "4"},
			Stdin: []byte("abcdefghij"),
		},
		{
			// R1.2: multiple lines.
			Name:  "multiple_lines",
			Args:  []string{"-w", "5"},
			Stdin: []byte("abc\nabcdefgh\nhi\n"),
		},
		{
			// R1.4: '-' means stdin.
			Name:  "dash_stdin",
			Args:  []string{"-w", "10", "-"},
			Stdin: []byte(strings.Repeat("f", 20) + "\n"),
		},
		{
			// R1.2: read from named file.
			Name:    "read_file",
			Args:    []string{shortFile},
			WorkDir: tmpDir,
		},
		{
			// R1.2: read from named file with wrapping.
			Name:    "read_file_wrap",
			Args:    []string{"-w", "20", longFile},
			WorkDir: tmpDir,
		},
		{
			// R1.1: empty input.
			Name:  "empty_input",
			Stdin: []byte(""),
		},
		{
			// R1.2: empty lines pass through.
			Name:  "empty_lines",
			Stdin: []byte("\n\n\n"),
		},
		{
			// R1.3: tab character column counting.
			Name:  "tab_column_counting",
			Args:  []string{"-w", "10"},
			Stdin: []byte("a\tb\n"),
		},
		{
			// R1.3: tab at boundary.
			Name:  "tab_at_boundary",
			Args:  []string{"-w", "8"},
			Stdin: []byte("\thello\n"),
		},
		{
			// R1.1: width of 1.
			Name:  "width_one",
			Args:  []string{"-w", "1"},
			Stdin: []byte("abc\n"),
		},
		{
			// R1.4: file interspersed with stdin via '-'.
			Name:    "file_and_dash",
			Args:    []string{shortFile, "-"},
			Stdin:   []byte("from stdin\n"),
			WorkDir: tmpDir,
		},
		// R2.3: byte counting mode (-b flag).
		{
			// R2.3: -b counts bytes, not columns.
			Name:  "byte_mode_basic",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("abcdefghij"),
		},
		{
			// R2.3: -b with newline-terminated input.
			Name:  "byte_mode_with_newline",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("abcdefghij\n"),
		},
		{
			// R2.3: -b disables tab-stop expansion; tab is 1 byte.
			Name:  "byte_mode_tab_as_one_byte",
			Args:  []string{"-b", "-w", "4"},
			Stdin: []byte("a\tbcd\n"),
		},
		{
			// R2.3: -b with default width (80 bytes).
			Name:  "byte_mode_default_width",
			Args:  []string{"-b"},
			Stdin: []byte(strings.Repeat("x", 100) + "\n"),
		},
		{
			// R2.3: -b with width 1 wraps every byte.
			Name:  "byte_mode_width_one",
			Args:  []string{"-b", "-w", "1"},
			Stdin: []byte("abc\n"),
		},
		{
			// R2.3: -b with multibyte UTF-8 character splits mid-character.
			Name:  "byte_mode_multibyte_utf8",
			Args:  []string{"-b", "-w", "2"},
			Stdin: []byte("a\xc3\xa9b\n"),
		},
		{
			// R2.3: -b combined short flag form -bw4.
			Name:  "byte_mode_combined_flags",
			Args:  []string{"-bw4"},
			Stdin: []byte("abcdefgh\n"),
		},
		{
			// R2.3: -b with long line requiring multiple wraps.
			Name:  "byte_mode_triple_wrap",
			Args:  []string{"-b", "-w", "5"},
			Stdin: []byte(strings.Repeat("z", 17) + "\n"),
		},
		// R3.1-R3.4: space-break mode (-s flag) and interaction with -b.
		{
			// R3.1: -s breaks at last space.
			Name:  "space_break_basic",
			Args:  []string{"-s", "-w", "11"},
			Stdin: []byte("hello world foo bar\n"),
		},
		{
			// R3.2: -s falls back to exact break when no space.
			Name:  "space_break_no_space",
			Args:  []string{"-s", "-w", "5"},
			Stdin: []byte("abcdefghij\n"),
		},
		{
			// R3.4: -b -s combines byte counting with space breaking.
			Name:  "byte_mode_space_break",
			Args:  []string{"-b", "-s", "-w", "11"},
			Stdin: []byte("hello world foo bar\n"),
		},
		{
			// R3.4: -b -s with no space in segment.
			Name:  "byte_mode_space_break_no_space",
			Args:  []string{"-b", "-s", "-w", "5"},
			Stdin: []byte("abcdefghij\n"),
		},
		{
			// R3.3: space is last char on line before newline.
			Name:  "space_break_space_position",
			Args:  []string{"-s", "-w", "6"},
			Stdin: []byte("aa bb cc dd\n"),
		},
		{
			// R3.4: -bs combined short flags.
			Name:  "byte_space_combined_short",
			Args:  []string{"-bs", "-w", "11"},
			Stdin: []byte("hello world foo bar\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
