// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tac against gtac (GNU coreutils).
// Implements prd021-tac R1.1-R1.4, R2.1-R2.4, R4.1-R4.3 test coverage.
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
	refBin, err := exec.LookPath("gtac")
	if err != nil {
		t.Skipf("reference binary gtac not in PATH: %v", err)
	}

	// Create test fixtures in a temp directory.
	tmpDir := t.TempDir()
	writeTestFile(t, tmpDir, "three-lines.txt", "a\nb\nc\n")
	writeTestFile(t, tmpDir, "no-trailing-newline.txt", "a\nb\nc")
	writeTestFile(t, tmpDir, "single-line.txt", "hello\n")
	writeTestFile(t, tmpDir, "empty.txt", "")
	writeTestFile(t, tmpDir, "file1.txt", "x\ny\n")
	writeTestFile(t, tmpDir, "file2.txt", "1\n2\n")
	writeTestFile(t, tmpDir, "comma1.txt", "a,b,c,")
	writeTestFile(t, tmpDir, "comma2.txt", "x,y,")

	tests := []testutils.DiffTest{
		// R1.1: reverse lines from stdin with trailing newline.
		{
			Name:  "R1.1_reverse_stdin_trailing_newline",
			Stdin: []byte("a\nb\nc\n"),
		},
		// R1.1: reverse lines from a named file.
		{
			Name:    "R1.1_reverse_named_file",
			Args:    []string{filepath.Join(tmpDir, "three-lines.txt")},
			WorkDir: tmpDir,
		},
		// R1.2: trailing newline is terminator, not separator before empty record.
		// "a\nb\n" reversed → "b\na\n", not "\nb\na".
		{
			Name:  "R1.2_trailing_newline_terminator",
			Stdin: []byte("a\nb\n"),
		},
		// R1.2: no trailing newline preserved.
		// "a\nb\nc" reversed → "c\nb\na" (no trailing newline).
		{
			Name:  "R1.2_no_trailing_newline",
			Stdin: []byte("a\nb\nc"),
		},
		// R1.3: read from stdin when no arguments.
		{
			Name:  "R1.3_stdin_no_args",
			Stdin: []byte("first\nsecond\nthird\n"),
		},
		// R1.3: "-" means stdin.
		{
			Name:  "R1.3_dash_means_stdin",
			Args:  []string{"-"},
			Stdin: []byte("alpha\nbeta\n"),
		},
		// R1.4: multiple files processed independently.
		{
			Name: "R1.4_multiple_files_independent",
			Args: []string{
				filepath.Join(tmpDir, "file1.txt"),
				filepath.Join(tmpDir, "file2.txt"),
			},
			WorkDir: tmpDir,
		},
		// Edge: single line with trailing newline.
		{
			Name:  "single_line_trailing_newline",
			Stdin: []byte("only\n"),
		},
		// Edge: single line without trailing newline.
		{
			Name:  "single_line_no_trailing_newline",
			Stdin: []byte("only"),
		},
		// Edge: empty input.
		{
			Name:  "empty_stdin",
			Stdin: []byte(""),
		},
		// Edge: empty file.
		{
			Name:    "empty_file",
			Args:    []string{filepath.Join(tmpDir, "empty.txt")},
			WorkDir: tmpDir,
		},
		// Edge: input with only newlines.
		{
			Name:  "only_newlines",
			Stdin: []byte("\n\n\n"),
		},
		// Edge: file without trailing newline.
		{
			Name:    "file_no_trailing_newline",
			Args:    []string{filepath.Join(tmpDir, "no-trailing-newline.txt")},
			WorkDir: tmpDir,
		},
		// R3.2: non-existent file exits 1.
		{
			Name:      "nonexistent_file_exit_1",
			Args:      []string{filepath.Join(tmpDir, "does-not-exist.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R3.2: non-existent mixed with existing — exit 1, still outputs
		// reversed content of existing files.
		{
			Name: "nonexistent_mixed",
			Args: []string{
				filepath.Join(tmpDir, "three-lines.txt"),
				filepath.Join(tmpDir, "does-not-exist.txt"),
				filepath.Join(tmpDir, "single-line.txt"),
			},
			WorkDir:   tmpDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},

		// R2.1: -s with comma separator reverses comma-separated records.
		{
			Name:  "R2.1_separator_comma",
			Args:  []string{"-s", ","},
			Stdin: []byte("a,b,c,"),
		},
		// R2.1: -s with colon separator.
		{
			Name:  "R2.1_separator_colon",
			Args:  []string{"-s", ":"},
			Stdin: []byte("a:b:c:"),
		},
		// R2.1: -s with multi-character separator.
		{
			Name:  "R2.1_separator_multichar",
			Args:  []string{"-s", "::"},
			Stdin: []byte("a::b::c::"),
		},
		// R2.1: -s with no trailing separator.
		{
			Name:  "R2.1_separator_no_trailing",
			Args:  []string{"-s", ","},
			Stdin: []byte("a,b,c"),
		},
		// R2.1: -s with single record (no separator in input).
		{
			Name:  "R2.1_separator_single_record",
			Args:  []string{"-s", ","},
			Stdin: []byte("hello"),
		},
		// R2.2: -b attaches separator before each record.
		{
			Name:  "R2.2_before_newline",
			Args:  []string{"-b"},
			Stdin: []byte("\na\nb\nc"),
		},
		// R2.3-R2.4: -b -s with colon separator.
		{
			Name:  "R2.3_before_colon",
			Args:  []string{"-b", "-s", ":"},
			Stdin: []byte(":a:b:c"),
		},
		// R2.4: -b -s with comma separator.
		{
			Name:  "R2.4_before_comma",
			Args:  []string{"-b", "-s", ","},
			Stdin: []byte(",x,y,z"),
		},
		// R2.4: -b -s with trailing content after last record.
		{
			Name:  "R2.4_before_comma_no_leading",
			Args:  []string{"-b", "-s", ","},
			Stdin: []byte("x,y,z"),
		},
		// R2.1: -s with newline separator is same as default.
		{
			Name:  "R2.1_separator_newline_explicit",
			Args:  []string{"-s", "\n"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R2.2: -b with default newline separator.
		{
			Name:  "R2.2_before_default_newline",
			Args:  []string{"-b", "-s", "\n"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R2.1: empty input with separator flag.
		{
			Name:  "R2.1_separator_empty_input",
			Args:  []string{"-s", ","},
			Stdin: []byte(""),
		},
		// R2.1: -s with file argument.
		{
			Name:    "R2.1_separator_file",
			Args:    []string{"-s", "\n", filepath.Join(tmpDir, "three-lines.txt")},
			WorkDir: tmpDir,
		},

		// R3.1: -r with character class separator matching single chars.
		{
			Name:  "R3.1_regex_separator_charclass",
			Args:  []string{"-r", "-s", "[,;]"},
			Stdin: []byte("a,b;c,"),
		},
		// R3.1: -r with literal colon separator (same as fixed string).
		{
			Name:  "R3.1_regex_separator_colon",
			Args:  []string{"-r", "-s", ":"},
			Stdin: []byte("a:b:c:"),
		},
		// R3.1: -r with dot separator (matches any single character).
		{
			Name:  "R3.1_regex_separator_dot",
			Args:  []string{"-r", "-s", "[0-9]"},
			Stdin: []byte("a1b2c3d"),
		},
		// R3.2: -r matched text attaches to preceding record.
		{
			Name:  "R3.2_regex_match_attaches_preceding",
			Args:  []string{"-r", "-s", "-"},
			Stdin: []byte("a-b-c-d"),
		},
		// R3.3: -r combined with -b (before mode with regex).
		{
			Name:  "R3.3_regex_before_mode",
			Args:  []string{"-r", "-b", "-s", ":"},
			Stdin: []byte(":a:b:c"),
		},
		// R3.3: -r -b with character class separator.
		{
			Name:  "R3.3_regex_before_charclass",
			Args:  []string{"-r", "-b", "-s", "[0-9]"},
			Stdin: []byte("1abc2def3ghi"),
		},
		// R3.4: invalid regex exits with error. Error messages differ between
		// GNU (POSIX regex) and Go (regexp package), so we normalize stderr
		// to just the program name prefix.
		{
			Name:      "R3.4_invalid_regex_exit",
			Args:      []string{"-r", "-s", "[invalid"},
			Stdin:     []byte("test"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeRegexError},
		},

		// R4.2: binary data in stdin (bytes outside printable ASCII).
		{
			Name:  "R4.2_binary_data",
			Stdin: []byte{0x00, 0x0A, 0xFF, 0x0A, 0x80, 0x0A},
		},
		// R4.2: binary data without trailing newline.
		{
			Name:  "R4.2_binary_data_no_trailing_newline",
			Stdin: []byte{0x01, 0x02, 0x0A, 0xFE, 0xFF},
		},
		// R4.2: very long lines (4 KB each).
		{
			Name:  "R4.2_very_long_lines",
			Stdin: buildLongLineInput(4096, 3),
		},
		// R4.2: multiple file operands with -s flag.
		{
			Name: "R4.2_multiple_files_with_separator",
			Args: []string{
				"-s", ",",
				filepath.Join(tmpDir, "comma1.txt"),
				filepath.Join(tmpDir, "comma2.txt"),
			},
			WorkDir: tmpDir,
		},
		// R4.3: -s with separator not present in input.
		{
			Name:  "R4.3_separator_not_in_input",
			Args:  []string{"-s", "|"},
			Stdin: []byte("no pipe chars here"),
		},
		// R4.3: all three flags together (-r -b -s).
		{
			Name:  "R4.3_all_flags_regex_before_sep",
			Args:  []string{"-r", "-b", "-s", "[|]"},
			Stdin: []byte("|alpha|beta|gamma"),
		},
		// R4.3: -r -s with alternation regex pattern.
		{
			Name:  "R4.3_regex_alternation",
			Args:  []string{"-r", "-s", "[,;]"},
			Stdin: []byte("a,b;c,d;"),
		},
		// R4.3: -b with multi-character separator.
		{
			Name:  "R4.3_before_multichar_sep",
			Args:  []string{"-b", "-s", "::"},
			Stdin: []byte("::x::y::z"),
		},
		// R4.3: -s with tab separator.
		{
			Name:  "R4.3_tab_separator",
			Args:  []string{"-s", "\t"},
			Stdin: []byte("a\tb\tc\t"),
		},
		// R4.3: -b -s with tab separator.
		{
			Name:  "R4.3_before_tab_separator",
			Args:  []string{"-b", "-s", "\t"},
			Stdin: []byte("\ta\tb\tc"),
		},
		// R4.2: lines of only separator characters.
		{
			Name:  "R4.2_only_separators",
			Stdin: []byte(",,,"),
			Args:  []string{"-s", ","},
		},
		// R4.2: single-character input (no separator).
		{
			Name:  "R4.2_single_char_no_separator",
			Stdin: []byte("x"),
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
// GNU tac reports errors as "gtac: file: Error" while our binary uses
// "tac: file: error". This normalizer replaces the program name and lowercases
// the output to eliminate both differences.
func normalizeProgramName(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gtac: "), []byte("tac: "))
	return bytes.ToLower(b)
}

// buildLongLineInput creates input with n lines, each lineLen bytes of 'A',
// separated by newlines with a trailing newline.
func buildLongLineInput(lineLen, n int) []byte {
	line := bytes.Repeat([]byte("A"), lineLen)
	var buf bytes.Buffer
	for range n {
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// normalizeRegexError normalizes regex error messages for differential
// comparison. GNU tac uses POSIX regex error messages while Go uses its own
// format. Both produce a non-empty error line to stderr; we normalize to a
// canonical form so the messages can be compared.
func normalizeRegexError(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gtac: "), []byte("tac: "))
	// Replace everything after "tac: " with a canonical error marker so
	// different regex error message formats still match.
	if bytes.Contains(b, []byte("tac: ")) {
		return []byte("tac: regex error\n")
	}
	return b
}
