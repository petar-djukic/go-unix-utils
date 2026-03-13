// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/head.
//
// Implements: prd018-head R1.1–R1.5, R3.1–R3.2, R4.1–R4.3
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

	// Non-existent file for error tests.
	missing := filepath.Join(dir, "nonexistent.txt")

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
		// R3.2: single file — no header.
		{
			Name: "r3.2_single_file_no_header",
			Args: []string{fiveLines},
			Env:  []string{"LC_ALL=C"},
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
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
