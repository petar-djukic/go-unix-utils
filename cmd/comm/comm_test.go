// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/comm against GNU gcomm.
// Covers prd029-comm R4.1-R4.4 (differential testing).
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// writeFile creates a file with the given content in dir and returns its path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", p, err)
	}
	return p
}

// stderrNormalizer normalizes error messages between GNU gcomm and Go comm.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?comm|gcomm`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	noSuch := regexp.MustCompile(`(?i)no such file or directory`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("comm"))
		b = tryHelp.ReplaceAll(b, nil)
		b = noSuch.ReplaceAll(b, []byte("No such file or directory"))
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	dir := t.TempDir()

	// Standard test files: file1='a\nb\nc\n', file2='b\nc\nd\n'
	f1 := writeFile(t, dir, "f1.txt", "a\nb\nc\n")
	f2 := writeFile(t, dir, "f2.txt", "b\nc\nd\n")

	// Edge-case files
	emptyFile := writeFile(t, dir, "empty.txt", "")
	identicalA := writeFile(t, dir, "id_a.txt", "x\ny\nz\n")
	identicalB := writeFile(t, dir, "id_b.txt", "x\ny\nz\n")
	disjointA := writeFile(t, dir, "disj_a.txt", "a\nc\ne\n")
	disjointB := writeFile(t, dir, "disj_b.txt", "b\nd\nf\n")
	shortFile := writeFile(t, dir, "short.txt", "a\n")
	longFile := writeFile(t, dir, "long.txt", "a\nb\nc\nd\ne\n")
	unsortedFile := writeFile(t, dir, "unsorted.txt", "c\na\nb\n")
	singleLine := writeFile(t, dir, "single.txt", "m\n")

	tests := []testutils.DiffTest{
		// --- R4.1: Basic three-column output ---

		// R1.1, R1.2: standard three-column merge of two sorted files.
		{
			Name: "three_column_basic",
			Args: []string{f1, f2},
		},
		// R1.3: file1 exhausted first; remaining file2 lines go to column 2.
		{
			Name: "file1_exhausted_first",
			Args: []string{shortFile, longFile},
		},
		// R1.3: file2 exhausted first; remaining file1 lines go to column 1.
		{
			Name: "file2_exhausted_first",
			Args: []string{longFile, shortFile},
		},

		// --- R4.2: Column suppression flags ---

		// R2.1: -1 suppresses column 1 (lines unique to file1).
		{
			Name: "suppress_col1",
			Args: []string{"-1", f1, f2},
		},
		// R2.2: -2 suppresses column 2 (lines unique to file2).
		{
			Name: "suppress_col2",
			Args: []string{"-2", f1, f2},
		},
		// R2.3: -3 suppresses column 3 (common lines).
		{
			Name: "suppress_col3",
			Args: []string{"-3", f1, f2},
		},
		// R2.1+R2.2: -12 shows only common lines (column 3).
		{
			Name: "suppress_col12",
			Args: []string{"-12", f1, f2},
		},
		// R2.1+R2.3: -13 shows only file2-unique lines (column 2).
		{
			Name: "suppress_col13",
			Args: []string{"-13", f1, f2},
		},
		// R2.2+R2.3: -23 shows only file1-unique lines (column 1).
		{
			Name: "suppress_col23",
			Args: []string{"-23", f1, f2},
		},
		// R2.1+R2.2+R2.3: -123 suppresses all columns (no output).
		{
			Name: "suppress_col123",
			Args: []string{"-123", f1, f2},
		},
		// R2.4: tab alignment with -1 flag on disjoint files.
		{
			Name: "suppress_col1_disjoint",
			Args: []string{"-1", disjointA, disjointB},
		},
		// R2.4: tab alignment with -2 flag on disjoint files.
		{
			Name: "suppress_col2_disjoint",
			Args: []string{"-2", disjointA, disjointB},
		},
		// Separate -1 -2 flags (equivalent to -12).
		{
			Name: "suppress_separate_1_2",
			Args: []string{"-1", "-2", f1, f2},
		},

		// --- R4.3: --output-delimiter, --check-order, --nocheck-order, --total ---

		// R3.4: custom output delimiter replaces tab.
		{
			Name: "output_delimiter_pipe",
			Args: []string{"--output-delimiter=|", f1, f2},
		},
		// R3.4: custom output delimiter with multi-char string.
		{
			Name: "output_delimiter_multi",
			Args: []string{"--output-delimiter=::", f1, f2},
		},
		// R3.4: empty output delimiter.
		{
			Name: "output_delimiter_empty",
			Args: []string{"--output-delimiter=", f1, f2},
		},
		// R3.4: output delimiter combined with column suppression.
		{
			Name: "output_delimiter_with_suppress",
			Args: []string{"--output-delimiter=|", "-1", f1, f2},
		},
		// R3.3: --nocheck-order on unsorted input suppresses warnings.
		{
			Name: "nocheck_order_unsorted",
			Args:      []string{"--nocheck-order", unsortedFile, f2},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R3.1: default order checking on sorted input (no warning).
		{
			Name: "default_order_sorted",
			Args: []string{f1, f2},
		},
		// R3.2: --check-order on unsorted input produces error.
		{
			Name:      "check_order_unsorted",
			Args:      []string{"--check-order", unsortedFile, f2},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R3.1: default order checking on unsorted input (warns).
		{
			Name:      "default_order_unsorted",
			Args:      []string{unsortedFile, f2},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// --total flag appends summary counts.
		{
			Name: "total_flag",
			Args: []string{"--total", f1, f2},
		},
		// --total with column suppression.
		{
			Name: "total_with_suppress",
			Args: []string{"--total", "-1", f1, f2},
		},
		// --total with custom delimiter.
		{
			Name: "total_with_delimiter",
			Args: []string{"--total", "--output-delimiter=|", f1, f2},
		},

		// --- R4.4: Edge cases ---

		// Empty file as file1.
		{
			Name: "empty_file1",
			Args: []string{emptyFile, f2},
		},
		// Empty file as file2.
		{
			Name: "empty_file2",
			Args: []string{f1, emptyFile},
		},
		// Both files empty.
		{
			Name: "both_empty",
			Args: []string{emptyFile, emptyFile},
		},
		// Stdin via '-' as file1.
		{
			Name:  "stdin_as_file1",
			Args:  []string{"-", f2},
			Stdin: []byte("a\nb\nc\n"),
		},
		// Stdin via '-' as file2.
		{
			Name:  "stdin_as_file2",
			Args:  []string{f1, "-"},
			Stdin: []byte("b\nc\nd\n"),
		},
		// Identical files — all lines in column 3.
		{
			Name: "identical_files",
			Args: []string{identicalA, identicalB},
		},
		// Disjoint files — no common lines.
		{
			Name: "disjoint_files",
			Args: []string{disjointA, disjointB},
		},
		// Single-line files.
		{
			Name: "single_line_files",
			Args: []string{singleLine, singleLine},
		},
		// Single-line vs multi-line.
		{
			Name: "single_vs_multi",
			Args: []string{singleLine, longFile},
		},
		// Nonexistent file — error exit.
		{
			Name:      "nonexistent_file",
			Args:      []string{"/nonexistent-path/no-such-file.txt", f2},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// Empty stdin via '-'.
		{
			Name:  "empty_stdin",
			Args:  []string{"-", f2},
			Stdin: []byte{},
		},
		// --total with empty files.
		{
			Name: "total_empty_files",
			Args: []string{"--total", emptyFile, emptyFile},
		},
		// -12 with identical files (common only).
		{
			Name: "suppress_12_identical",
			Args: []string{"-12", identicalA, identicalB},
		},
		// -3 with disjoint files (unique only).
		{
			Name: "suppress_3_disjoint",
			Args: []string{"-3", disjointA, disjointB},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
