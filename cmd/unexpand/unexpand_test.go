// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/unexpand against the GNU reference binary (gunexpand).
// Implements prd025-unexpand R1.1-R1.4 test coverage.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgNameNormalizer replaces the reference binary name (gunexpand) with
// the Go binary name (unexpand) in stderr so error message comparisons match.
func stderrProgNameNormalizer(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gunexpand:"), []byte("unexpand:"))
}

// stderrCaseNormalizer lowercases stderr so platform-specific error message
// casing differences (e.g., "No such file" vs "no such file") do not cause
// false divergence.
func stderrCaseNormalizer(data []byte) []byte {
	return bytes.ToLower(data)
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunexpand")
	if err != nil {
		t.Skipf("reference binary gunexpand not in PATH: %v", err)
	}

	// Create temp files for file-input tests.
	tmpDir := t.TempDir()

	// File with leading spaces (8 spaces = one tab at default stops).
	leadingSpacesFile := filepath.Join(tmpDir, "leading.txt")
	if err := os.WriteFile(leadingSpacesFile, []byte("        hello\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// File with no spaces (passthrough).
	noSpaceFile := filepath.Join(tmpDir, "nospace.txt")
	if err := os.WriteFile(noSpaceFile, []byte("hello\tworld\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// File with mixed leading and embedded spaces.
	mixedFile := filepath.Join(tmpDir, "mixed.txt")
	if err := os.WriteFile(mixedFile, []byte("        hello        world\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Multi-file test inputs.
	fileA := filepath.Join(tmpDir, "a.txt")
	if err := os.WriteFile(fileA, []byte("        line1\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	fileB := filepath.Join(tmpDir, "b.txt")
	if err := os.WriteFile(fileB, []byte("        line2\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Empty file.
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte{}, 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Nonexistent file path for error tests.
	nonexistentFile := filepath.Join(tmpDir, "nonexistent.txt")

	tests := []testutils.DiffTest{
		// --- R1.1: Default leading-space-to-tab conversion ---
		{
			// R1.1: 8 leading spaces become one tab.
			Name:  "leading_8_spaces_to_tab",
			Stdin: []byte("        hello\n"),
		},
		{
			// R1.1: 16 leading spaces become two tabs.
			Name:  "leading_16_spaces_to_tabs",
			Stdin: []byte("                hello\n"),
		},
		{
			// R1.1: 4 leading spaces (not a full tab stop) stay as spaces.
			Name:  "leading_4_spaces_unchanged",
			Stdin: []byte("    hello\n"),
		},
		{
			// R1.1: 12 leading spaces = one tab + 4 spaces.
			Name:  "leading_12_spaces_partial",
			Stdin: []byte("            hello\n"),
		},
		{
			// R1.1: stdin input with no file args.
			Name:  "stdin_default",
			Stdin: []byte("        text\n"),
		},
		{
			// R1.1: empty input produces no output.
			Name:  "empty_input",
			Stdin: []byte{},
		},
		{
			// R1.1: read from named file.
			Name: "read_file",
			Args: []string{leadingSpacesFile},
		},
		{
			// R1.1: read from multiple files.
			Name: "multiple_files",
			Args: []string{fileA, fileB},
		},
		{
			// R1.1: '-' means stdin.
			Name:  "dash_stdin",
			Args:  []string{"-"},
			Stdin: []byte("        from stdin\n"),
		},
		{
			// R1.1: file and '-' interspersed.
			Name:  "file_and_dash",
			Args:  []string{fileA, "-"},
			Stdin: []byte("        from stdin\n"),
		},

		// --- R1.2: Non-leading whitespace unchanged in default mode ---
		{
			// R1.2: spaces after first non-blank pass through unchanged.
			Name:  "non_leading_spaces_unchanged",
			Stdin: []byte("hello        world\n"),
		},
		{
			// R1.2: mixed leading conversion and non-leading passthrough.
			Name:  "leading_converted_non_leading_unchanged",
			Stdin: []byte("        hello        world\n"),
		},
		{
			// R1.2: single space between words unchanged.
			Name:  "single_space_between_words",
			Stdin: []byte("a b\n"),
		},
		{
			// R1.2: no leading spaces, embedded spaces unchanged.
			Name:  "no_leading_embedded_unchanged",
			Stdin: []byte("abc        def\n"),
		},
		{
			// R1.2: tabs after non-blank pass through in default mode.
			Name:  "embedded_tab_passthrough",
			Stdin: []byte("hello\tworld\n"),
		},

		// --- R1.3: Spaces that don't reach a tab stop stay as spaces ---
		{
			// R1.3: 3 leading spaces (short of tab stop 8).
			Name:  "partial_spaces_3",
			Stdin: []byte("   hello\n"),
		},
		{
			// R1.3: 7 leading spaces (short of tab stop 8).
			Name:  "partial_spaces_7",
			Stdin: []byte("       hello\n"),
		},
		{
			// R1.3: 1 leading space.
			Name:  "single_leading_space",
			Stdin: []byte(" hello\n"),
		},
		{
			// R1.3: 9 leading spaces = one tab + 1 space.
			Name:  "nine_leading_spaces",
			Stdin: []byte("         hello\n"),
		},

		// --- R1.4: Existing tabs in leading whitespace ---
		{
			// R1.4: existing leading tab passes through.
			Name:  "existing_leading_tab",
			Stdin: []byte("\thello\n"),
		},
		{
			// R1.4: tab followed by spaces — spaces continue from tab stop.
			Name:  "tab_then_spaces",
			Stdin: []byte("\t        hello\n"),
		},
		{
			// R1.4: spaces then tab — tab advances to next stop.
			Name:  "spaces_then_tab",
			Stdin: []byte("    \thello\n"),
		},
		{
			// R1.4: multiple leading tabs.
			Name:  "multiple_leading_tabs",
			Stdin: []byte("\t\thello\n"),
		},
		{
			// R1.4: mixed spaces and tabs in leading whitespace.
			Name:  "mixed_leading_spaces_tabs",
			Stdin: []byte("    \t    hello\n"),
		},

		// --- R1.3 (-a/--all flag): Convert all space sequences ---
		{
			// R1.3: -a converts non-leading spaces.
			Name:  "all_mode_non_leading",
			Args:  []string{"-a"},
			Stdin: []byte("hello        world\n"),
		},
		{
			// R1.3: -a with leading and embedded spaces.
			Name:  "all_mode_leading_and_embedded",
			Args:  []string{"-a"},
			Stdin: []byte("        hello        world\n"),
		},
		{
			// R1.3: --all long form.
			Name:  "all_long_form",
			Args:  []string{"--all"},
			Stdin: []byte("hello        world\n"),
		},
		{
			// R1.3: -a with single space between words (not at tab stop).
			Name:  "all_mode_single_space",
			Args:  []string{"-a"},
			Stdin: []byte("a b\n"),
		},
		{
			// R1.3: -a across multiple lines.
			Name:  "all_mode_multiline",
			Args:  []string{"-a"},
			Stdin: []byte("a        b\nc        d\n"),
		},
		{
			// R1.3: -a with file input.
			Name: "all_mode_file",
			Args: []string{"-a", mixedFile},
		},

		// --- R1.4 (--first-only flag): Default behavior ---
		{
			// R1.4: --first-only is the default — only leading converted.
			Name:  "first_only_explicit",
			Args:  []string{"--first-only"},
			Stdin: []byte("        hello        world\n"),
		},
		{
			// R1.4: -a then --first-only — last wins, default behavior.
			Name:  "a_then_first_only",
			Args:  []string{"-a", "--first-only"},
			Stdin: []byte("        hello        world\n"),
		},
		{
			// R1.4: --first-only then -a — last wins, all mode.
			Name:  "first_only_then_a",
			Args:  []string{"--first-only", "-a"},
			Stdin: []byte("        hello        world\n"),
		},
		{
			// R1.4: multiple -a and --first-only — last wins.
			Name:  "multiple_flag_precedence",
			Args:  []string{"-a", "--first-only", "-a", "--first-only"},
			Stdin: []byte("        hello        world\n"),
		},

		// --- Edge cases ---
		{
			// Edge: single newline.
			Name:  "single_newline",
			Stdin: []byte("\n"),
		},
		{
			// Edge: multiple empty lines.
			Name:  "multiple_empty_lines",
			Stdin: []byte("\n\n\n"),
		},
		{
			// Edge: no trailing newline.
			Name:  "no_trailing_newline",
			Stdin: []byte("        hello"),
		},
		{
			// Edge: only spaces (no newline).
			Name:  "only_spaces_no_newline",
			Stdin: []byte("        "),
		},
		{
			// Edge: only tabs.
			Name:  "only_tabs",
			Stdin: []byte("\t\t\t\n"),
		},
		{
			// Edge: spaces and tabs alternating in leading whitespace.
			Name:  "alternating_space_tab",
			Stdin: []byte(" \t \t hello\n"),
		},
		{
			// Edge: long line with many leading spaces.
			Name:  "long_leading_spaces",
			Stdin: []byte(strings.Repeat(" ", 80) + "text\n"),
		},
		{
			// Edge: empty file.
			Name: "empty_file",
			Args: []string{emptyFile},
		},
		{
			// Edge: binary content with null bytes.
			Name:  "binary_content",
			Stdin: []byte("\x00        \x01\n"),
		},
		{
			// Edge: CRLF line endings.
			Name:  "crlf_line_endings",
			Stdin: []byte("        hello\r\n"),
		},
		{
			// Edge: -a with long line of spaces between words.
			Name:  "all_mode_long_spaces",
			Args:  []string{"-a"},
			Stdin: []byte("a" + strings.Repeat(" ", 40) + "b\n"),
		},

		// --- Error handling ---
		{
			// Error: nonexistent file.
			Name:      "error_nonexistent_file",
			Args:      []string{nonexistentFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer, stderrCaseNormalizer},
		},
		{
			// Error: nonexistent file among valid files — processing continues.
			Name:      "error_nonexistent_with_valid",
			Args:      []string{leadingSpacesFile, nonexistentFile, noSpaceFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer, stderrCaseNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
