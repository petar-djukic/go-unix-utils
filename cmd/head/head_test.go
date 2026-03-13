// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/head.
//
// Implements: prd018-head R1.1–R1.5, R2.1–R2.3, R3.1–R3.5, R4.1–R4.4
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const binGhead = "ghead"

// headErrRe matches head/ghead error lines and normalizes the program name
// and format differences. GNU head uses "cannot open '...' for reading: ..."
// while Go's os.Open produces "open ...: ...".
var headErrRe = regexp.MustCompile(`(?m)^g?head: (?:cannot open '|open )(.+?)(?:'? for reading)?: .+$`)

// normalizeHeadErrors replaces head/ghead error lines with a canonical form.
func normalizeHeadErrors(b []byte) []byte {
	return headErrRe.ReplaceAll(b, []byte("PROG: $1: ERROR"))
}

// normalizeVersionOutput replaces all version output with a canonical string.
// GNU ghead and Go head produce different version strings; this normalizer
// ensures the differential test compares only that non-empty output was produced.
func normalizeVersionOutput(b []byte) []byte {
	if len(b) > 0 {
		return []byte("VERSION_OUTPUT\n")
	}
	return b
}

// normalizeHelpOutput replaces all help output with a canonical string.
// GNU ghead and Go head produce different help text; this normalizer
// ensures the differential test compares only that non-empty output was produced.
func normalizeHelpOutput(b []byte) []byte {
	if len(b) > 0 {
		return []byte("HELP_OUTPUT\n")
	}
	return b
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(binGhead)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", binGhead, err)
	}

	// Create temp files for file-based test cases.
	dir := t.TempDir()

	// 20-line file for default and explicit -n tests.
	twentyLines := filepath.Join(dir, "twenty.txt")
	var twentyContent strings.Builder
	for i := 1; i <= 20; i++ {
		twentyContent.WriteString(strings.Repeat("x", i) + "\n")
	}
	if err := os.WriteFile(twentyLines, []byte(twentyContent.String()), 0o644); err != nil {
		t.Fatalf("writing twenty.txt: %v", err)
	}

	// 5-line file for multi-file tests.
	fiveLines := filepath.Join(dir, "five.txt")
	if err := os.WriteFile(fiveLines, []byte("a\nb\nc\nd\ne\n"), 0o644); err != nil {
		t.Fatalf("writing five.txt: %v", err)
	}

	// 3-line file for multi-file tests.
	threeLines := filepath.Join(dir, "three.txt")
	if err := os.WriteFile(threeLines, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatalf("writing three.txt: %v", err)
	}

	// File with no trailing newline.
	noTrailingNL := filepath.Join(dir, "notrail.txt")
	if err := os.WriteFile(noTrailingNL, []byte("alpha\nbeta"), 0o644); err != nil {
		t.Fatalf("writing notrail.txt: %v", err)
	}

	// Binary-ish content file for byte-mode tests.
	byteFile := filepath.Join(dir, "bytes.txt")
	if err := os.WriteFile(byteFile, []byte("abcdefghijklmnopqrstuvwxyz"), 0o644); err != nil {
		t.Fatalf("writing bytes.txt: %v", err)
	}

	// Non-existent file for error tests.
	missing := filepath.Join(dir, "nonexistent.txt")

	// R3.2: unreadable file for permission-denied tests.
	unreadable := filepath.Join(dir, "noperm.txt")
	if err := os.WriteFile(unreadable, []byte("secret\n"), 0o000); err != nil {
		t.Fatalf("writing noperm.txt: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: default 10 lines from stdin.
		{
			Name:  "r1.1_default_10_lines_stdin",
			Stdin: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: default 10 lines from a file.
		{
			Name: "r1.1_default_10_lines_file",
			Args: []string{twentyLines},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: fewer lines than default — prints all lines.
		{
			Name:  "r1.1_fewer_than_10_lines",
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: explicit -n 5.
		{
			Name:  "r1.3_explicit_n_5",
			Args:  []string{"-n", "5"},
			Stdin: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: explicit -n 1.
		{
			Name:  "r1.3_explicit_n_1",
			Args:  []string{"-n", "1"},
			Stdin: []byte("first\nsecond\nthird\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: -n larger than input.
		{
			Name:  "r1.3_n_larger_than_input",
			Args:  []string{"-n", "100"},
			Stdin: []byte("only\nthree\nlines\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: stdin when no arguments.
		{
			Name:  "r1.4_stdin_no_args",
			Stdin: []byte("hello\nworld\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: "-" means stdin.
		{
			Name:  "r1.4_dash_means_stdin",
			Args:  []string{"-"},
			Stdin: []byte("from stdin\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.5: last line without trailing newline is still counted.
		{
			Name: "r1.5_no_trailing_newline",
			Args: []string{noTrailingNL},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.5: stdin without trailing newline.
		{
			Name:  "r1.5_stdin_no_trailing_newline",
			Args:  []string{"-n", "3"},
			Stdin: []byte("a\nb\nc"),
			Env:   []string{"LC_ALL=C"},
		},

		// R2.1: byte-count mode from stdin.
		{
			Name:  "r2.1_byte_mode_stdin",
			Args:  []string{"-c", "5"},
			Stdin: []byte("abcdefghij"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: byte-count mode from file.
		{
			Name: "r2.1_byte_mode_file",
			Args: []string{"-c", "10", byteFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -c 0 produces no output.
		{
			Name:  "r2.1_byte_mode_zero",
			Args:  []string{"-c", "0"},
			Stdin: []byte("should not appear"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -c larger than input.
		{
			Name:  "r2.1_byte_mode_larger_than_input",
			Args:  []string{"-c", "100"},
			Stdin: []byte("short"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: negative byte count — all but last N bytes.
		{
			Name:  "r2.2_negative_byte_count",
			Args:  []string{"-c", "-5"},
			Stdin: []byte("abcdefghij"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: negative byte count larger than input.
		{
			Name:  "r2.2_negative_byte_count_larger",
			Args:  []string{"-c", "-100"},
			Stdin: []byte("tiny"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: negative byte count from file.
		{
			Name: "r2.2_negative_byte_count_file",
			Args: []string{"-c", "-6", byteFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: byte-count with K suffix.
		{
			Name:  "r2.3_byte_mode_suffix_K",
			Args:  []string{"-c", "1K"},
			Stdin: []byte(strings.Repeat("x", 2048)),
			Env:   []string{"LC_ALL=C"},
		},

		// R3.1: multiple files get headers.
		{
			Name: "r3.1_multi_file_headers",
			Args: []string{fiveLines, threeLines},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: multiple files with -n.
		{
			Name: "r3.1_multi_file_with_n",
			Args: []string{"-n", "2", fiveLines, threeLines},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: multiple files with -c.
		{
			Name: "r3.1_multi_file_with_c",
			Args: []string{"-c", "5", fiveLines, threeLines},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: single file — no header.
		{
			Name: "r3.2_single_file_no_header",
			Args: []string{fiveLines},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: -q suppresses headers for multiple files.
		{
			Name: "r3.3_quiet_suppresses_headers",
			Args: []string{"-q", fiveLines, threeLines},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: -q with -n.
		{
			Name: "r3.3_quiet_with_n",
			Args: []string{"-q", "-n", "2", fiveLines, threeLines},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: -v forces header for single file.
		{
			Name: "r3.4_verbose_single_file",
			Args: []string{"-v", fiveLines},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: -v with stdin.
		{
			Name:  "r3.4_verbose_stdin",
			Args:  []string{"-v"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R4.1: exit 0 on success.
		{
			Name:     "r4.1_exit0_success",
			Args:     []string{fiveLines},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R4.2: exit 1 on missing file.
		{
			Name:      "r4.2_exit1_missing_file",
			Args:      []string{missing},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeHeadErrors},
		},
		// R4.2: missing file followed by existing file — exit 1, continues processing.
		{
			Name:      "r4.2_missing_then_existing",
			Args:      []string{missing, fiveLines},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeHeadErrors},
		},
		// R1.3: -n 0 produces no output.
		{
			Name:  "r1.3_n_zero",
			Args:  []string{"-n", "0"},
			Stdin: []byte("should not appear\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Empty stdin — produces no output.
		{
			Name:  "empty_stdin",
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},

		// R3.1: nonexistent file — error to stderr, exit 1.
		{
			Name:      "r3.1_nonexistent_file_error",
			Args:      []string{missing},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeHeadErrors},
		},
		// R3.2: unreadable file (permission denied) — error to stderr, exit 1.
		{
			Name:      "r3.2_permission_denied",
			Args:      []string{unreadable},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeHeadErrors},
		},
		// R3.3: mixed valid and invalid files — processes valid, errors for invalid, exit 1.
		{
			Name:      "r3.3_mixed_valid_invalid",
			Args:      []string{fiveLines, missing, threeLines},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeHeadErrors},
		},
		// R3.3: valid file then unreadable file — exit 1.
		{
			Name:      "r3.3_valid_then_unreadable",
			Args:      []string{fiveLines, unreadable},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeHeadErrors},
		},
		// R3.3: unreadable file then valid file — exit 1, continues to valid.
		{
			Name:      "r3.3_unreadable_then_valid",
			Args:      []string{unreadable, fiveLines},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeHeadErrors},
		},
		// R3.4: multiple nonexistent files — all errors, exit 1.
		{
			Name:      "r3.4_multiple_nonexistent",
			Args:      []string{missing, filepath.Join(dir, "also_missing.txt")},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeHeadErrors},
		},

		// R3.5: error on unopenable file, continues to remaining files.
		{
			Name:      "r3.5_error_continues_remaining",
			Args:      []string{missing, fiveLines},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeHeadErrors},
		},

		// R4.1: --version prints version info to stdout and exits 0.
		{
			Name:      "r4.1_version_flag",
			Args:      []string{"--version"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeVersionOutput},
		},

		// R4.2: --help prints usage to stdout and exits 0.
		{
			Name:      "r4.2_help_flag",
			Args:      []string{"--help"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeHelpOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
