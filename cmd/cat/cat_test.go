// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cat against gcat (GNU coreutils).
//
// Covers prd006-cat R2.4, R3.1, R3.2, R3.3.
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
	refBin, err := exec.LookPath("gcat")
	if err != nil {
		t.Skip("reference binary gcat not in PATH")
	}

	tests := []testutils.DiffTest{
		// R2.4: blank line is only \n; lines with spaces/tabs are not blank
		{
			Name:  "R2.4_blank_vs_whitespace_with_b",
			Args:  []string{"-b"},
			Stdin: []byte("a\n\n \n\t\nb\n"),
		},
		// R3.1: suppress repeated blank lines
		{
			Name:  "R3.1_squeeze_repeated_blanks",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		{
			Name:  "R3.1_squeeze_single_blank_kept",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\nb\n"),
		},
		{
			Name:  "R3.1_squeeze_three_blanks",
			Args:  []string{"-s"},
			Stdin: []byte("\n\n\na\n\n\n\nb\n\n\n"),
		},
		// R3.2: squeeze across file boundaries
		{
			Name:     "R3.2_squeeze_across_files",
			Args:     []string{"-s"},
			ExitCode: 0,
		},
		// R3.3: squeeze with -n (squeeze before numbering)
		{
			Name:  "R3.3_squeeze_with_n",
			Args:  []string{"-sn"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		// R3.3: squeeze with -b
		{
			Name:  "R3.3_squeeze_with_b",
			Args:  []string{"-sb"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		// R2.4: ensure spaces-only line is not blank for -b
		{
			Name:  "R2.4_space_line_numbered_by_b",
			Args:  []string{"-b"},
			Stdin: []byte("a\n \nb\n"),
		},
		// R3.1: no blanks, no effect
		{
			Name:  "R3.1_no_blanks_no_effect",
			Args:  []string{"-s"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R3.3: squeeze with both -n and -b (b overrides n)
		{
			Name:  "R3.3_squeeze_with_bn",
			Args:  []string{"-sbn"},
			Stdin: []byte("x\n\n\n\ny\n"),
		},
	}

	// R3.2: create temp files for cross-boundary squeeze test
	setupCrossBoundaryTest(t, tests)

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// setupCrossBoundaryTest creates temp files for the R3.2 cross-boundary test.
func setupCrossBoundaryTest(t *testing.T, tests []testutils.DiffTest) {
	t.Helper()
	for i := range tests {
		if tests[i].Name != "R3.2_squeeze_across_files" {
			continue
		}
		dir := t.TempDir()
		file1 := filepath.Join(dir, "f1.txt")
		file2 := filepath.Join(dir, "f2.txt")
		writeTestFile(t, file1, "a\n\n")
		writeTestFile(t, file2, "\nb\n")
		tests[i].Args = []string{"-s", file1, file2}
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
