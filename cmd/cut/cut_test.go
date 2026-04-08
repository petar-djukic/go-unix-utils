// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/cut: differential testing against gcut.
// Implements srd026-cut R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R4.1, R4.2, R4.3, R4.4.
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

// binaryPathRe matches full path references to gcut (e.g., /opt/homebrew/bin/gcut).
var binaryPathRe = regexp.MustCompile(`/[^\s']*gcut`)

// binaryNameNorm normalizes binary names in error messages so
// reference binary (gcut) and Go binary compare equally.
// Handles full paths like '/opt/homebrew/bin/gcut --help' → 'cut --help'.
var binaryNameNorm testutils.NormalizeFunc = func(b []byte) []byte {
	b = binaryPathRe.ReplaceAll(b, []byte("cut"))
	b = bytes.ReplaceAll(b, []byte("gcut:"), []byte("cut:"))
	return b
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcut")
	if err != nil {
		t.Skipf("reference binary gcut not in PATH: %v", err)
	}

	errNorm := []testutils.NormalizeFunc{binaryNameNorm}

	tests := []testutils.DiffTest{
		// === R1.1: byte selection ===
		{Name: "byte_single", Args: []string{"-b", "2"}, Stdin: []byte("abcdef\n")},
		{Name: "byte_range", Args: []string{"-b", "2-4"}, Stdin: []byte("abcdef\n")},
		{Name: "byte_open_end", Args: []string{"-b", "3-"}, Stdin: []byte("abcdef\n")},
		{Name: "byte_open_start", Args: []string{"-b", "-3"}, Stdin: []byte("abcdef\n")},
		{Name: "byte_comma_list", Args: []string{"-b", "1,3,5"}, Stdin: []byte("abcdef\n")},
		{Name: "byte_mixed", Args: []string{"-b", "1,3-5"}, Stdin: []byte("abcdefgh\n")},
		{Name: "byte_overlapping", Args: []string{"-b", "1-3,2-5"}, Stdin: []byte("abcdefgh\n")},
		{Name: "byte_inline", Args: []string{"-b2-4"}, Stdin: []byte("abcdef\n")},
		{Name: "bytes_long", Args: []string{"--bytes=2-4"}, Stdin: []byte("abcdef\n")},

		// === R1.2: character selection (equivalent to bytes under LC_ALL=C) ===
		{Name: "char_range", Args: []string{"-c", "2-4"}, Stdin: []byte("abcdef\n")},
		{Name: "char_single", Args: []string{"-c", "1"}, Stdin: []byte("hello\n")},
		{Name: "char_inline", Args: []string{"-c1,3"}, Stdin: []byte("abcde\n")},
		{Name: "chars_long", Args: []string{"--characters=2-4"}, Stdin: []byte("abcdef\n")},

		// === R1.3: newlines pass through ===
		{Name: "multiline", Args: []string{"-b", "1-2"}, Stdin: []byte("ab\ncd\nef\n")},
		{Name: "multiline_byte", Args: []string{"-b", "2"}, Stdin: []byte("abc\ndef\nghi\n")},

		// === R1.4: short lines ===
		{Name: "short_line", Args: []string{"-b", "1-10"}, Stdin: []byte("abc\n")},
		{Name: "short_high_range", Args: []string{"-b", "5-"}, Stdin: []byte("ab\n")},
		{Name: "empty_line", Args: []string{"-b", "1"}, Stdin: []byte("\n")},
		{Name: "no_trailing_newline", Args: []string{"-b", "1-2"}, Stdin: []byte("abcdef")},

		// === R2.1: field selection ===
		{Name: "field_single", Args: []string{"-d", ":", "-f", "2"}, Stdin: []byte("a:b:c\n")},
		{Name: "field_range", Args: []string{"-d", ":", "-f", "1,3"}, Stdin: []byte("a:b:c\n")},
		{Name: "field_open_end", Args: []string{"-d", ":", "-f", "2-"}, Stdin: []byte("a:b:c:d\n")},
		{Name: "field_open_start", Args: []string{"-d", ":", "-f", "-2"}, Stdin: []byte("a:b:c\n")},
		{Name: "field_tab_default", Args: []string{"-f", "2"}, Stdin: []byte("a\tb\tc\n")},
		{Name: "field_inline", Args: []string{"-d:", "-f2"}, Stdin: []byte("a:b:c\n")},
		{Name: "fields_long", Args: []string{"--fields=1,3", "-d", ":"}, Stdin: []byte("a:b:c\n")},

		// === R2.2: delimiter ===
		{Name: "delim_comma", Args: []string{"-d", ",", "-f", "1,3"}, Stdin: []byte("x,y,z\n")},
		{Name: "delim_space", Args: []string{"-d", " ", "-f", "2"}, Stdin: []byte("hello world\n")},
		{Name: "delim_long", Args: []string{"--delimiter=:", "-f", "2"}, Stdin: []byte("a:b:c\n")},

		// === R2.3: suppress lines without delimiter ===
		{Name: "suppress_no_delim", Args: []string{"-d", ":", "-f", "2", "-s"}, Stdin: []byte("no-delimiter\n")},
		{Name: "suppress_with_delim", Args: []string{"-d", ":", "-f", "2", "-s"}, Stdin: []byte("a:b:c\n")},
		{Name: "suppress_mixed", Args: []string{"-d", ":", "-f", "2", "-s"}, Stdin: []byte("no-delim\na:b:c\nalso-no\nx:y:z\n")},
		{Name: "no_suppress", Args: []string{"-d", ":", "-f", "2"}, Stdin: []byte("no-delimiter\n")},
		{Name: "suppress_long", Args: []string{"--only-delimited", "-d", ":", "-f", "2"}, Stdin: []byte("no-delimiter\n")},

		// === R2.4: output delimiter ===
		{Name: "outdelim_field", Args: []string{"-d", ":", "-f", "1,3", "--output-delimiter", "|"}, Stdin: []byte("a:b:c\n")},
		{Name: "outdelim_field_eq", Args: []string{"-d:", "-f1,3", "--output-delimiter=|"}, Stdin: []byte("a:b:c\n")},
		{Name: "outdelim_field_multi", Args: []string{"-d", ":", "-f", "1-3", "--output-delimiter", " - "}, Stdin: []byte("a:b:c\n")},
		{Name: "outdelim_byte", Args: []string{"-b", "1,3", "--output-delimiter", ":"}, Stdin: []byte("abcdef\n")},
		{Name: "outdelim_byte_range", Args: []string{"-b", "1-2,5-6", "--output-delimiter", "|"}, Stdin: []byte("abcdefgh\n")},
		{Name: "outdelim_char", Args: []string{"-c", "1,3,5", "--output-delimiter", ","}, Stdin: []byte("abcdef\n")},
		{Name: "outdelim_byte_adjacent", Args: []string{"-b", "1-3", "--output-delimiter", ":"}, Stdin: []byte("abcdef\n")},

		// === R3.1: complement mode ===
		{Name: "complement_byte", Args: []string{"-b", "2-4", "--complement"}, Stdin: []byte("abcdef\n")},
		{Name: "complement_char", Args: []string{"-c", "1,3", "--complement"}, Stdin: []byte("abcde\n")},
		{Name: "complement_field", Args: []string{"-d", ":", "-f", "2", "--complement"}, Stdin: []byte("a:b:c\n")},
		// R3.3: complement with -f preserves field order
		{Name: "complement_field_multi", Args: []string{"-d", ":", "-f", "1,3", "--complement"}, Stdin: []byte("a:b:c:d\n")},
		{Name: "complement_outdelim", Args: []string{"-d", ":", "-f", "2", "--complement", "--output-delimiter", "|"}, Stdin: []byte("a:b:c\n")},
		{Name: "complement_byte_outdelim", Args: []string{"-b", "2-4", "--complement", "--output-delimiter", ":"}, Stdin: []byte("abcdef\n")},

		// === R4.2: stdin handling ===
		{Name: "stdin_dash", Args: []string{"-b", "1-3", "-"}, Stdin: []byte("hello\n")},
		{Name: "stdin_no_args", Args: []string{"-b", "1-3"}, Stdin: []byte("hello\n")},
		{Name: "stdin_after_ddash", Args: []string{"-b", "1-3", "--", "-"}, Stdin: []byte("hello\n")},

		// === Error cases (R4.3) ===
		{Name: "err_no_list", Args: []string{}, ExitCode: 1, Normalize: errNorm},
		{Name: "err_both_bf", Args: []string{"-b", "1", "-f", "1"}, ExitCode: 1, Normalize: errNorm},
		{Name: "err_both_bc", Args: []string{"-b", "1", "-c", "1"}, ExitCode: 1, Normalize: errNorm},
		{Name: "err_both_cf", Args: []string{"-c", "1", "-f", "1"}, ExitCode: 1, Normalize: errNorm},
		{Name: "err_decreasing_range", Args: []string{"-b", "5-3"}, ExitCode: 1, Normalize: errNorm},
		{Name: "err_delim_not_field", Args: []string{"-b", "1", "-d", ":"}, ExitCode: 1, Normalize: errNorm},
		{Name: "err_suppress_not_field", Args: []string{"-b", "1", "-s"}, ExitCode: 1, Normalize: errNorm},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffMultiFile tests multi-file input processing.
// R4.2: sequential processing of multiple file arguments.
func TestDiffMultiFile(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcut")
	if err != nil {
		t.Skipf("reference binary gcut not in PATH: %v", err)
	}

	dir := t.TempDir()
	file1 := filepath.Join(dir, "a.txt")
	file2 := filepath.Join(dir, "b.txt")
	writeTestFile(t, file1, "a:b:c\n")
	writeTestFile(t, file2, "x:y:z\n")

	tests := []testutils.DiffTest{
		{Name: "two_files", Args: []string{"-d", ":", "-f", "2", file1, file2}},
		{Name: "file_and_stdin", Args: []string{"-d", ":", "-f", "2", file1, "-"}, Stdin: []byte("p:q:r\n")},
		{Name: "stdin_and_file", Args: []string{"-d", ":", "-f", "2", "-", file2}, Stdin: []byte("p:q:r\n")},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFileError tests error handling for nonexistent files.
// R4.2: exit 1 on file open failure, processing continues for remaining files.
func TestDiffFileError(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcut")
	if err != nil {
		t.Skipf("reference binary gcut not in PATH: %v", err)
	}

	dir := t.TempDir()
	goodFile := filepath.Join(dir, "good.txt")
	writeTestFile(t, goodFile, "a:b:c\n")
	badFile := filepath.Join(dir, "nonexistent.txt")

	errNorm := []testutils.NormalizeFunc{binaryNameNorm}

	tests := []testutils.DiffTest{
		{
			Name:      "nonexistent_file",
			Args:      []string{"-d", ":", "-f", "2", badFile},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "good_then_bad",
			Args:      []string{"-d", ":", "-f", "2", goodFile, badFile},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "bad_then_good",
			Args:      []string{"-d", ":", "-f", "2", badFile, goodFile},
			ExitCode:  1,
			Normalize: errNorm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile creates a file with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file %s: %v", path, err)
	}
}
