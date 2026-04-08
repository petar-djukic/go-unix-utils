// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/comm against gcomm.
// Implements srd029-comm R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R3.4
// acceptance criteria via testutils.RunDiffTests.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeStderr replaces the reference binary name and normalizes
// error message casing so differential comparison succeeds.
func normalizeStderr(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("gcomm:"), []byte("comm:"))
	data = bytes.ReplaceAll(data,
		[]byte("No such file or directory"),
		[]byte("no such file or directory"))
	return data
}

// normalizeStderrHint strips the "Try '...' for more information." line
// that GNU comm appends to error messages.
func normalizeStderrHint(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	var out [][]byte
	for _, l := range lines {
		if bytes.HasPrefix(l, []byte("Try '")) {
			continue
		}
		out = append(out, l)
	}
	return bytes.Join(out, []byte("\n"))
}

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestDiff runs differential tests for comm against gcomm.
// D2: uses testutils.BuildBinary and exec.LookPath with t.Skip.
// D3: LC_ALL=C is set by default via testutils.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	dir := t.TempDir()
	f1 := writeTestFile(t, dir, "file1.txt", "a\nb\nc\n")
	f2 := writeTestFile(t, dir, "file2.txt", "b\nc\nd\n")
	empty := writeTestFile(t, dir, "empty.txt", "")
	same := writeTestFile(t, dir, "same.txt", "a\nb\nc\n")

	// R3.1/R3.2: unsorted input for order-checking tests.
	// file1 unsorted: "a\nc\nb\nd\n" has c before b (out of order).
	// file2 sorted: "a\nb\nc\nd\n" triggers comparison deep enough
	// that the merge loop detects file1's disorder.
	unsorted1 := writeTestFile(t, dir, "unsorted1.txt", "a\nc\nb\nd\n")
	sorted2 := writeTestFile(t, dir, "sorted2.txt", "a\nb\nc\nd\n")

	stderrNorm := []testutils.NormalizeFunc{normalizeStderr, normalizeStderrHint}

	tests := []testutils.DiffTest{
		{
			// AC1: basic three-column output
			Name: "basic_three_columns",
			Args: []string{f1, f2},
		},
		{
			// AC2: suppress columns 1 and 2, show only common lines
			Name: "suppress_col1_col2",
			Args: []string{"-12", f1, f2},
		},
		{
			// R2.1: suppress column 1 only
			Name: "suppress_col1",
			Args: []string{"-1", f1, f2},
		},
		{
			// R2.2: suppress column 2 only
			Name: "suppress_col2",
			Args: []string{"-2", f1, f2},
		},
		{
			// AC3: suppress column 3, show only unique lines
			Name: "suppress_col3",
			Args: []string{"-3", f1, f2},
		},
		{
			// R2.3: suppress all three columns, no output
			Name: "suppress_all",
			Args: []string{"-123", f1, f2},
		},
		{
			// AC4: R3.4 custom output delimiter
			Name: "output_delimiter",
			Args: []string{"--output-delimiter=|", f1, f2},
		},
		{
			// R1.3: file1 is empty, all lines from file2 in col2
			Name: "empty_file1",
			Args: []string{empty, f2},
		},
		{
			// R1.3: file2 is empty, all lines from file1 in col1
			Name: "empty_file2",
			Args: []string{f1, empty},
		},
		{
			// R1.2: identical files, all lines in column 3
			Name: "identical_files",
			Args: []string{f1, same},
		},
		{
			// R4.2: nonexistent file produces error
			Name:      "nonexistent_file",
			Args:      []string{f1, filepath.Join(dir, "no_such_file.txt")},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			// R1.1: missing operand produces error
			Name:      "missing_operand",
			Args:      []string{},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			// AC6: reading file2 from stdin via '-'
			Name:  "stdin_as_file2",
			Args:  []string{f1, "-"},
			Stdin: []byte("b\nc\nd\n"),
		},
		{
			// AC6: reading file1 from stdin via '-'
			Name:  "stdin_as_file1",
			Args:  []string{"-", f2},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R2.1 + R2.4: suppress col1, col2 shifts left (no indent)
			Name: "suppress_col1_indentation",
			Args: []string{"-1", f1, f2},
		},
		{
			// R2.2 + R2.4: suppress col2, col3 shifts left (one tab)
			Name: "suppress_col2_indentation",
			Args: []string{"-2", f1, f2},
		},
		{
			// R2.3 + R2.1: suppress col1 and col3, only col2 remains
			Name: "suppress_col1_col3",
			Args: []string{"-13", f1, f2},
		},
		{
			// R2.3 + R2.2: suppress col2 and col3, only col1 remains
			Name: "suppress_col2_col3",
			Args: []string{"-23", f1, f2},
		},
		// --- R3 order-checking and delimiter tests ---
		{
			// AC5/R3.2: --check-order with unsorted file1 produces error
			Name:      "check_order_unsorted",
			Args:      []string{"--check-order", unsorted1, sorted2},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			// R3.3: --nocheck-order with unsorted input succeeds
			Name: "nocheck_order_unsorted",
			Args: []string{"--nocheck-order", unsorted1, sorted2},
		},
		{
			// R3.1: default behavior with unsorted input warns but continues
			Name:      "default_order_unsorted",
			Args:      []string{unsorted1, sorted2},
			Normalize: stderrNorm,
		},
		{
			// R3.4: --output-delimiter with multi-char string
			Name: "output_delimiter_multi_char",
			Args: []string{"--output-delimiter=<=>", f1, f2},
		},
		{
			// R3.4: --output-delimiter combined with column suppression
			Name: "output_delimiter_with_suppress",
			Args: []string{"--output-delimiter=,", "-1", f1, f2},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
