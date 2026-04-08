// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/comm against gcomm.
// Implements srd029-comm acceptance criteria via testutils.RunDiffTests.
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
			// AC4: custom output delimiter
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
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
