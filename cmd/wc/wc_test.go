// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/wc against gwc (GNU wc from Homebrew coreutils).
// Implements: prd005-wc R1.1–R1.4, R2.1–R2.6, R3.1–R3.3, R4.1–R4.4, R5.1–R5.2, R6.1 differential testing.
// R5.1: All differential tests set LC_ALL=C to avoid locale-dependent divergence.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeBinaryName replaces "gwc:" with "wc:" in stderr so the differential
// test does not fail on the binary name prefix difference.
var normalizeBinaryName testutils.NormalizeFunc = func(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gwc:"), []byte("wc:"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skipf("reference binary gwc not in PATH: %v", err)
	}

	// Create test fixture files in a temp directory.
	tmpDir := t.TempDir()

	// file1: simple two-line file
	file1 := filepath.Join(tmpDir, "file1.txt")
	writeTestFile(t, file1, "foo\nbar baz\n")

	// file2: single line no trailing newline
	file2 := filepath.Join(tmpDir, "file2.txt")
	writeTestFile(t, file2, "hello world")

	// empty: zero-byte file
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	writeTestFile(t, emptyFile, "")

	// binary: contains arbitrary bytes
	binaryFile := filepath.Join(tmpDir, "binary.dat")
	writeTestFileBytes(t, binaryFile, []byte{0x00, 0x01, 0xFF, 0x0A, 0x41, 0x42})

	// tabfile: contains tabs for -L testing
	tabFile := filepath.Join(tmpDir, "tabs.txt")
	writeTestFile(t, tabFile, "a\tb\n12345678c\n")

	// files0: NUL-delimited file list for --files0-from testing (R4.4)
	files0File := filepath.Join(tmpDir, "filelist.txt")
	writeTestFileBytes(t, files0File, []byte(file1+"\x00"+file2+"\x00"))

	// files0Single: NUL-delimited file list with single entry
	files0Single := filepath.Join(tmpDir, "filelist_single.txt")
	writeTestFileBytes(t, files0Single, []byte(file1+"\x00"))

	tests := []testutils.DiffTest{
		// === R1.1–R1.4: default behavior (no flags) ===
		// R1.1, R1.2: stdin with default flags
		{
			Name:  "stdin_default",
			Stdin: []byte("foo\nbar baz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: empty stdin
		{
			Name:  "stdin_empty",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: single line no trailing newline
		{
			Name:  "stdin_no_trailing_newline",
			Stdin: []byte("hello world"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: single named file
		{
			Name: "single_file",
			Args: []string{file1},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: multiple named files with total line
		{
			Name: "multi_file",
			Args: []string{file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: "-" as stdin alongside named files
		{
			Name:  "dash_stdin_with_file",
			Args:  []string{"-", file1},
			Stdin: []byte("one two three\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: total line with three files
		{
			Name: "three_files_total",
			Args: []string{file1, file2, emptyFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.2: binary input
		{
			Name:  "binary_stdin",
			Stdin: []byte{0x00, 0x01, 0xFF, 0x0A, 0x41, 0x42},
			Env:   []string{"LC_ALL=C"},
		},
		// R4.3: empty file
		{
			Name: "empty_file",
			Args: []string{emptyFile},
			Env:  []string{"LC_ALL=C"},
		},
		// binary file
		{
			Name: "binary_file",
			Args: []string{binaryFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: stdin with multiple words and lines
		{
			Name:  "stdin_multiline",
			Stdin: []byte("line one\nline two\nline three\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: tab and space mixing
		{
			Name:  "stdin_tabs_spaces",
			Stdin: []byte("word1\tword2  word3\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: dash only (stdin as file arg)
		{
			Name:  "dash_only",
			Args:  []string{"-"},
			Stdin: []byte("a b c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Non-existent file
		{
			Name:      "nonexistent_file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// Non-existent file mixed with valid file
		{
			Name:      "nonexistent_mixed",
			Args:      []string{file1, filepath.Join(tmpDir, "nonexistent.txt")},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},

		// === R2.1: -l flag (line count only) ===
		{
			Name:  "flag_l_stdin",
			Args:  []string{"-l"},
			Stdin: []byte("foo\nbar\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name: "flag_l_file",
			Args: []string{"-l", file1},
			Env:  []string{"LC_ALL=C"},
		},

		// === R2.2: -w flag (word count only) ===
		{
			Name:  "flag_w_stdin",
			Args:  []string{"-w"},
			Stdin: []byte("foo bar baz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name: "flag_w_file",
			Args: []string{"-w", file1},
			Env:  []string{"LC_ALL=C"},
		},

		// === R2.3: -c flag (byte count only) ===
		{
			Name:  "flag_c_stdin",
			Args:  []string{"-c"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name: "flag_c_file",
			Args: []string{"-c", file1},
			Env:  []string{"LC_ALL=C"},
		},

		// === R2.4: -m flag (char count) ===
		{
			Name:  "flag_m_stdin",
			Args:  []string{"-m"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name: "flag_m_file",
			Args: []string{"-m", file1},
			Env:  []string{"LC_ALL=C"},
		},

		// === R2.3: -c and -m together (-m takes precedence) ===
		{
			Name:  "flag_cm_stdin",
			Args:  []string{"-c", "-m"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "flag_mc_stdin",
			Args:  []string{"-m", "-c"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// === R2.5: -L flag (max line length) ===
		{
			Name:  "flag_L_stdin",
			Args:  []string{"-L"},
			Stdin: []byte("short\na longer line\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name: "flag_L_file",
			Args: []string{"-L", file1},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "flag_L_tabs",
			Args: []string{"-L", tabFile},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name:  "flag_L_no_trailing_newline",
			Args:  []string{"-L"},
			Stdin: []byte("no newline"),
			Env:   []string{"LC_ALL=C"},
		},

		// === R2.6: combined flags — column order is lines, words, chars/bytes, max-line-length ===
		{
			Name:  "flag_lw_stdin",
			Args:  []string{"-l", "-w"},
			Stdin: []byte("foo\nbar baz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "flag_wl_stdin",
			Args:  []string{"-w", "-l"},
			Stdin: []byte("foo\nbar baz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "flag_lc_stdin",
			Args:  []string{"-l", "-c"},
			Stdin: []byte("foo\nbar baz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "flag_lwc_stdin",
			Args:  []string{"-l", "-w", "-c"},
			Stdin: []byte("foo\nbar baz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Combined short flags (-lw as single arg)
		{
			Name:  "flag_combined_lw",
			Args:  []string{"-lw"},
			Stdin: []byte("foo\nbar baz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "flag_combined_lwc",
			Args:  []string{"-lwc"},
			Stdin: []byte("foo\nbar baz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// -L combined with other flags
		{
			Name:  "flag_lL_stdin",
			Args:  []string{"-l", "-L"},
			Stdin: []byte("short\na longer line\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "flag_lwcL_stdin",
			Args:  []string{"-l", "-w", "-c", "-L"},
			Stdin: []byte("foo\nbar baz\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// === R2.3: multi-file with selection flags ===
		{
			Name: "flag_l_multi_file",
			Args: []string{"-l", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "flag_w_multi_file",
			Args: []string{"-w", file1, file2, emptyFile},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "flag_lw_multi_file",
			Args: []string{"-lw", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "flag_L_multi_file",
			Args: []string{"-L", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "flag_lwcL_multi_file",
			Args: []string{"-lwcL", file1, file2, emptyFile},
			Env:  []string{"LC_ALL=C"},
		},

		// === R4.3: empty input with flags ===
		{
			Name:  "flag_l_empty_stdin",
			Args:  []string{"-l"},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name: "flag_c_empty_file",
			Args: []string{"-c", emptyFile},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "flag_L_empty_file",
			Args: []string{"-L", emptyFile},
			Env:  []string{"LC_ALL=C"},
		},

		// === R2.5: -L tab expansion and display columns ===
		// R2.5: tab at position 0 advances to column 8.
		{
			Name:  "R2.5_tab_expansion",
			Args:  []string{"-L"},
			Stdin: []byte("\tX\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.5: -L combined with -l shows both columns.
		{
			Name:  "R2.5_L_with_l",
			Args:  []string{"-lL"},
			Stdin: []byte("short\na much longer line here\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// === R2.6: combined flag output order verification ===
		// R2.6: -Lwc must output in order: words, bytes, max-line-length (not flag order).
		{
			Name:  "R2.6_Lwc_order",
			Args:  []string{"-Lwc"},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.6: all flags combined in reverse order must still produce lines, words, bytes, max-line-length.
		{
			Name:  "R2.6_Lcwl_order",
			Args:  []string{"-Lcwl"},
			Stdin: []byte("abc\ndef ghi\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// === R3.1: multi-file column alignment ===
		// R3.1: columns must be wide enough for the largest count across all files.
		{
			Name: "R3.1_alignment_multi_file",
			Args: []string{file1, file2, emptyFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: multi-file with -lw flag to verify alignment of subset columns.
		{
			Name: "R3.1_alignment_lw_multi",
			Args: []string{"-lw", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},

		// === R3.2: total line with "total" label ===
		// R3.2: total line sums counts from all files.
		{
			Name: "R3.2_total_line",
			Args: []string{file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: total line with three files including empty.
		{
			Name: "R3.2_total_three_files",
			Args: []string{"-lwc", file1, file2, emptyFile},
			Env:  []string{"LC_ALL=C"},
		},

		// === R3.3: --total flag modes ===
		// R3.3: --total=always with single file — prints file + total.
		{
			Name: "R3.3_total_always_single",
			Args: []string{"--total=always", file1},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: --total=always with multiple files — same as default multi-file.
		{
			Name: "R3.3_total_always_multi",
			Args: []string{"--total=always", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: --total=only with single file — prints only the total line.
		{
			Name: "R3.3_total_only_single",
			Args: []string{"--total=only", file1},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: --total=only with multiple files — prints only the total line.
		{
			Name: "R3.3_total_only_multi",
			Args: []string{"--total=only", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: --total=never with single file — no total (same as default single).
		{
			Name: "R3.3_total_never_single",
			Args: []string{"--total=never", file1},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: --total=never with multiple files — no total line.
		{
			Name: "R3.3_total_never_multi",
			Args: []string{"--total=never", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: --total=auto explicit — same as default.
		{
			Name: "R3.3_total_auto_multi",
			Args: []string{"--total=auto", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: --total=always with "-" as file arg.
		{
			Name:  "R3.3_total_always_dash",
			Args:  []string{"--total=always", "-"},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.3: --total=always combined with selection flags.
		{
			Name: "R3.3_total_always_lw",
			Args: []string{"-lw", "--total=always", file1},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: --total=only with three files.
		{
			Name: "R3.3_total_only_three_files",
			Args: []string{"--total=only", file1, file2, emptyFile},
			Env:  []string{"LC_ALL=C"},
		},

		// === R4.1: "-" treated as stdin file argument ===
		// R4.1: single dash as file argument reads stdin and labels as "-".
		{
			Name:  "R4.1_dash_stdin",
			Args:  []string{"-"},
			Stdin: []byte("a b c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.1: dash with named file shows total.
		{
			Name:  "R4.1_dash_with_file",
			Args:  []string{"-", file1},
			Stdin: []byte("one two three\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// === R4.2: binary input ===
		// R4.2: binary stdin with selection flags.
		{
			Name:  "R4.2_binary_stdin_lc",
			Args:  []string{"-lc"},
			Stdin: []byte{0x00, 0xFF, 0x0A, 0x80, 0x41},
			Env:   []string{"LC_ALL=C"},
		},

		// === R4.3: empty input ===
		// R4.3: empty stdin with all flags.
		{
			Name:  "R4.3_empty_stdin_lwcL",
			Args:  []string{"-lwcL"},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.3: empty file with default flags.
		{
			Name: "R4.3_empty_file_default",
			Args: []string{emptyFile},
			Env:  []string{"LC_ALL=C"},
		},

		// === R4.4: --files0-from ===
		// R4.4: read NUL-delimited filenames from a file.
		{
			Name: "R4.4_files0_from_file",
			Args: []string{"--files0-from=" + files0File},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.4: --files0-from with selection flags.
		{
			Name: "R4.4_files0_from_lw",
			Args: []string{"-lw", "--files0-from=" + files0File},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.4: --files0-from=- reads filenames from stdin.
		{
			Name:  "R4.4_files0_from_stdin",
			Args:  []string{"--files0-from=-"},
			Stdin: []byte(file1 + "\x00" + file2 + "\x00"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.4: --files0-from with single file in list.
		{
			Name: "R4.4_files0_from_single",
			Args: []string{"--files0-from=" + files0Single},
			Env:  []string{"LC_ALL=C"},
		},

		// === R5.1, R5.2: locale handling ===
		// R5.2: under LC_ALL=C, -m and -c produce identical counts.
		{
			Name:  "R5.2_m_equals_c_under_C_locale",
			Args:  []string{"-m"},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R5.2_c_under_C_locale",
			Args:  []string{"-c"},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R5.2: -m on file under LC_ALL=C matches -c.
		{
			Name: "R5.2_m_file_C_locale",
			Args: []string{"-m", file1},
			Env:  []string{"LC_ALL=C"},
		},

		// === R6.1: exit code 0 on success ===
		// R6.1: successful processing exits 0 (ExitCode defaults to 0).
		{
			Name:  "R6.1_exit_0_stdin",
			Stdin: []byte("test\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name: "R6.1_exit_0_file",
			Args: []string{file1},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile writes content to path, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", path, err)
	}
}

// writeTestFileBytes writes raw bytes to path, failing the test on error.
func writeTestFileBytes(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", path, err)
	}
}
