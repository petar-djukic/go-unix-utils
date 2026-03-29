// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sort against gsort (GNU coreutils).
//
// Covers prd053-sort R1.1-R1.7, R2.1-R2.4, R3.1-R3.4.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsort")
	if err != nil {
		t.Skip("reference binary gsort not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1: default lexicographic sort by byte value
		{
			Name:  "R1.1_default_lexicographic",
			Stdin: []byte("banana\napple\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: already sorted input
		{
			Name:  "R1.1_already_sorted",
			Stdin: []byte("apple\nbanana\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: single line
		{
			Name:  "R1.1_single_line",
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: empty input
		{
			Name:  "R1.1_empty_input",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: input without trailing newline
		{
			Name:  "R1.1_no_trailing_newline",
			Stdin: []byte("banana\napple"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: duplicate lines
		{
			Name:  "R1.1_duplicate_lines",
			Stdin: []byte("b\na\nb\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: numeric strings sorted lexicographically (not numerically)
		{
			Name:  "R1.1_numeric_strings_lexicographic",
			Stdin: []byte("10\n2\n1\n20\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: uppercase vs lowercase (byte-value ordering)
		{
			Name:  "R1.1_case_ordering",
			Stdin: []byte("banana\nApple\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: stdin via no file args (implicit)
		{
			Name:  "R1.2_stdin_implicit",
			Stdin: []byte("c\na\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: stdin via explicit "-"
		{
			Name:  "R1.2_stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("c\na\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: multiple files combined
		{
			Name: "R1.3_multi_file",
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: reverse sort
		{
			Name:  "R1.4_reverse",
			Args:  []string{"-r"},
			Stdin: []byte("apple\nbanana\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: --reverse long option
		{
			Name:  "R1.4_reverse_long",
			Args:  []string{"--reverse"},
			Stdin: []byte("apple\nbanana\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: reverse with duplicates
		{
			Name:  "R1.4_reverse_duplicates",
			Args:  []string{"-r"},
			Stdin: []byte("b\na\nb\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.5: -u unique removes duplicate lines
		{
			Name:  "R1.5_unique",
			Args:  []string{"-u"},
			Stdin: []byte("b\na\nb\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.5: --unique long option
		{
			Name:  "R1.5_unique_long",
			Args:  []string{"--unique"},
			Stdin: []byte("b\na\nb\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.5: -u combined with -r
		{
			Name:  "R1.5_unique_reverse",
			Args:  []string{"-u", "-r"},
			Stdin: []byte("b\na\nb\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.5: -u with all unique lines (no change)
		{
			Name:  "R1.5_unique_all_distinct",
			Args:  []string{"-u"},
			Stdin: []byte("c\na\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.6: tested in TestOutputFile and TestOutputFileSameAsInput
		// (cannot use differential framework for file output comparison)
		// R1.7: -s stable sort
		{
			Name:  "R1.7_stable",
			Args:  []string{"-s"},
			Stdin: []byte("banana\napple\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.7: --stable long option
		{
			Name:  "R1.7_stable_long",
			Args:  []string{"--stable"},
			Stdin: []byte("banana\napple\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.7: -s with -r
		{
			Name:  "R1.7_stable_reverse",
			Args:  []string{"-s", "-r"},
			Stdin: []byte("banana\napple\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R2.1: -n numeric sort ---
		{
			Name:  "R2.1_numeric_basic",
			Args:  []string{"-n"},
			Stdin: []byte("10\n2\n1\n20\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.1_numeric_with_sign",
			Args:  []string{"-n"},
			Stdin: []byte("-5\n3\n-1\n0\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.1_numeric_decimal",
			Args:  []string{"-n"},
			Stdin: []byte("1.5\n1.2\n1.9\n1.0\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.1_numeric_long_option",
			Args:  []string{"--numeric-sort"},
			Stdin: []byte("10\n2\n1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.1_numeric_reverse",
			Args:  []string{"-n", "-r"},
			Stdin: []byte("10\n2\n1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.1_numeric_leading_blanks",
			Args:  []string{"-n"},
			Stdin: []byte("  10\n 2\n1\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R2.2: -h human-numeric sort ---
		{
			Name:  "R2.2_human_basic",
			Args:  []string{"-h"},
			Stdin: []byte("1K\n100\n1M\n500\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.2_human_suffixes",
			Args:  []string{"-h"},
			Stdin: []byte("5G\n5M\n5K\n5T\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.2_human_long_option",
			Args:  []string{"--human-numeric-sort"},
			Stdin: []byte("2K\n1K\n3K\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.2_human_decimal",
			Args:  []string{"-h"},
			Stdin: []byte("1.5K\n1K\n2K\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R2.3: -M month sort ---
		{
			Name:  "R2.3_month_basic",
			Args:  []string{"-M"},
			Stdin: []byte("MAR\nJAN\nFEB\nDEC\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.3_month_unknown_first",
			Args:  []string{"-M"},
			Stdin: []byte("XYZ\nJAN\nFEB\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.3_month_long_option",
			Args:  []string{"--month-sort"},
			Stdin: []byte("MAR\nJAN\nFEB\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.3_month_all_months",
			Args:  []string{"-M"},
			Stdin: []byte("DEC\nJUN\nMAR\nSEP\nJAN\nJUL\nAPR\nOCT\nFEB\nAUG\nMAY\nNOV\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R2.4: -V version sort ---
		{
			Name:  "R2.4_version_basic",
			Args:  []string{"-V"},
			Stdin: []byte("file10\nfile2\nfile1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.4_version_segments",
			Args:  []string{"-V"},
			Stdin: []byte("1.10\n1.2\n1.1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.4_version_long_option",
			Args:  []string{"--version-sort"},
			Stdin: []byte("file10\nfile2\nfile1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.4_version_mixed",
			Args:  []string{"-V"},
			Stdin: []byte("a2b\na10b\na1b\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R3.1: -t field separator ---
		{
			Name:  "R3.1_field_separator_colon",
			Args:  []string{"-t:", "-k2,2"},
			Stdin: []byte("b:2\na:3\nc:1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.1_field_separator_long",
			Args:  []string{"--field-separator=:", "-k2,2"},
			Stdin: []byte("b:2\na:3\nc:1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.1_field_separator_comma",
			Args:  []string{"-t,", "-k2,2"},
			Stdin: []byte("b,2\na,3\nc,1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.1_field_separator_space",
			Args:  []string{"-t ", "-k2,2"},
			Stdin: []byte("b 2\na 3\nc 1\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R3.2: -k key field ---
		{
			Name:  "R3.2_key_second_field",
			Args:  []string{"-k2,2", "-t:"},
			Stdin: []byte("b:2\na:1\nc:3\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.2_key_numeric_modifier",
			Args:  []string{"-k2,2n", "-t:"},
			Stdin: []byte("a:10\nb:2\nc:1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.2_key_reverse_modifier",
			Args:  []string{"-k2,2r", "-t:"},
			Stdin: []byte("a:1\nb:3\nc:2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.2_key_long_option",
			Args:  []string{"--key=2,2", "-t:"},
			Stdin: []byte("b:2\na:1\nc:3\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.2_key_default_separator",
			Args:  []string{"-k2,2"},
			Stdin: []byte("c 2\na 3\nb 1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.2_key_numeric_field",
			Args:  []string{"-k2,2n"},
			Stdin: []byte("a 10\nb 2\nc 1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.2_key_to_end_of_line",
			Args:  []string{"-k2"},
			Stdin: []byte("a z\nb x\nc y\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.2_unique_by_key",
			Args:  []string{"-u", "-k1,1", "-t:"},
			Stdin: []byte("a:2\na:1\nb:1\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R3.3: multiple -k options ---
		{
			Name:  "R3.3_multi_key_tiebreak",
			Args:  []string{"-k1,1", "-k2,2", "-t:"},
			Stdin: []byte("a:2\na:1\nb:1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.3_multi_key_first_wins",
			Args:  []string{"-k1,1", "-k2,2n", "-t:"},
			Stdin: []byte("b:1\na:10\na:2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.3_multi_key_three_fields",
			Args:  []string{"-k1,1", "-k2,2", "-k3,3", "-t:"},
			Stdin: []byte("a:b:2\na:b:1\na:a:3\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R3.4: -b ignore leading blanks ---
		{
			Name:  "R3.4_ignore_blanks",
			Args:  []string{"-b"},
			Stdin: []byte("  b\na\n  c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.4_ignore_blanks_long",
			Args:  []string{"--ignore-leading-blanks"},
			Stdin: []byte("  b\na\n  c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.4_ignore_blanks_with_key",
			Args:  []string{"-b", "-k2,2", "-t:"},
			Stdin: []byte("a:  z\nb:  a\nc:  m\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.4_blanks_key_modifier",
			Args:  []string{"-k2b,2", "-t:"},
			Stdin: []byte("a:  z\nb:  a\nc:  m\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	setupMultiFileTest(t, tests)
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestOutputFile verifies -o writes to a file.
// R1.6: -o FILE must write output to FILE.
func TestOutputFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "input.txt")
	outputFile := filepath.Join(dir, "output.txt")
	writeTestFile(t, inputFile, "banana\napple\ncherry\n")

	cmd := exec.Command(goBin, "-o", outputFile, inputFile)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sort -o failed: %v\n%s", err, out)
	}

	got, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	want := "apple\nbanana\ncherry\n"
	if string(got) != want {
		t.Errorf("output file:\ngot:  %q\nwant: %q", string(got), want)
	}
}

// TestOutputFileSameAsInput verifies -o FILE works when FILE is also an input.
// R1.6: FILE may be the same as an input file.
func TestOutputFileSameAsInput(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	file := filepath.Join(dir, "data.txt")
	writeTestFile(t, file, "banana\napple\ncherry\n")

	cmd := exec.Command(goBin, "-o", file, file)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sort -o same-file failed: %v\n%s", err, out)
	}

	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	want := "apple\nbanana\ncherry\n"
	if string(got) != want {
		t.Errorf("in-place sort:\ngot:  %q\nwant: %q", string(got), want)
	}
}

// setupMultiFileTest creates temp files for the R1.3 multi-file test.
func setupMultiFileTest(t *testing.T, tests []testutils.DiffTest) {
	t.Helper()
	for i := range tests {
		if tests[i].Name != "R1.3_multi_file" {
			continue
		}
		dir := t.TempDir()
		file1 := filepath.Join(dir, "f1.txt")
		file2 := filepath.Join(dir, "f2.txt")
		writeTestFile(t, file1, "cherry\napple\n")
		writeTestFile(t, file2, "banana\ndate\n")
		tests[i].Args = []string{file1, file2}
		tests[i].Stdin = nil
	}
}

// writeTestFile writes content to a file, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeTestFile %s: %v", path, err)
	}
}
