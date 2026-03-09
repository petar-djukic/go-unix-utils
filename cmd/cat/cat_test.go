// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cat against the gcat reference binary.
// Implements prd006-cat AC1-AC6 via pkg/testutils.RunDiffTests.
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
		t.Skipf("reference binary gcat not in PATH: %v", err)
	}

	// Create test fixture files.
	dir := t.TempDir()
	file1 := testutils.WriteTestFile(t, dir, "file1.txt", "hello\nworld\n")
	file2 := testutils.WriteTestFile(t, dir, "file2.txt", "aaa\n")
	file3 := testutils.WriteTestFile(t, dir, "file3.txt", "bbb\n")

	// Binary test file with all 256 byte values.
	allBytes := make([]byte, 256)
	for i := range 256 {
		allBytes[i] = byte(i)
	}
	binFile := testutils.WriteTestFileBytes(t, dir, "binary.bin", allBytes)

	// File for blank line tests.
	blankFile := testutils.WriteTestFile(t, dir, "blanks.txt", "a\n\n\n\nb\n")

	// File with non-printing characters.
	nonPrintFile := testutils.WriteTestFileBytes(t, dir, "nonprint.bin",
		[]byte{0x01, 0x09, 0x1b, 0x7f, 0x80, 0xff, 0x0a})

	// File with tabs.
	tabFile := testutils.WriteTestFile(t, dir, "tabs.txt", "col1\tcol2\tcol3\n")

	// File for -A test.
	showAllFile := testutils.WriteTestFileBytes(t, dir, "showall.bin",
		[]byte{0x01, 0x09, 'h', 'e', 'l', 'l', 'o', 0x0a})

	// File for combined flag tests.
	numberedBlanks := testutils.WriteTestFile(t, dir, "nb.txt", "alpha\n\nbeta\n")

	// File for -e test.
	eFile := testutils.WriteTestFileBytes(t, dir, "efile.bin",
		[]byte{0x01, 'h', 'e', 'l', 'l', 'o', 0x0a})

	// File for -t test.
	tFile := testutils.WriteTestFileBytes(t, dir, "tfile.bin",
		[]byte{0x01, 0x09, 'h', 'e', 'l', 'l', 'o', 0x0a})

	// File for error recovery test.
	realFile := testutils.WriteTestFile(t, dir, "real.txt", "data\n")
	nonexistent := filepath.Join(dir, "nonexistent.txt")

	tests := []testutils.DiffTest{
		// R1.1, R1.5: default passthrough.
		{
			Name: "cat_default_passthrough",
			Args: []string{file1},
		},
		// R1.4: binary passthrough.
		{
			Name: "cat_binary_passthrough",
			Stdin: allBytes,
		},
		// R1.4: binary passthrough via file.
		{
			Name: "cat_binary_file_passthrough",
			Args: []string{binFile},
		},
		// R1.2: stdin when no args.
		{
			Name: "cat_stdin_no_args",
			Stdin: []byte("from stdin\n"),
		},
		// R1.2: "-" reads stdin.
		{
			Name: "cat_stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("from stdin\n"),
		},
		// R1.1, R1.3: multiple files.
		{
			Name: "cat_multiple_files",
			Args: []string{file2, file3},
		},
		// R2.1: line numbering -n.
		{
			Name:  "cat_line_numbering_n",
			Args:  []string{"-n"},
			Stdin: []byte("alpha\n\nbeta\n"),
		},
		// R2.1: -n with file.
		{
			Name: "cat_line_numbering_n_file",
			Args: []string{"-n", numberedBlanks},
		},
		// R2.2, R2.4: line numbering -b.
		{
			Name:  "cat_line_numbering_b",
			Args:  []string{"-b"},
			Stdin: []byte("first\n\n\nsecond\n"),
		},
		// R2.3: -b overrides -n.
		{
			Name:  "cat_b_overrides_n",
			Args:  []string{"-n", "-b"},
			Stdin: []byte("first\n\nsecond\n"),
		},
		// R3.1: squeeze blanks.
		{
			Name:  "cat_squeeze_blanks",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		// R3.1: squeeze with file.
		{
			Name: "cat_squeeze_blanks_file",
			Args: []string{"-s", blankFile},
		},
		// R3.3, R4.9: -n -s combined.
		{
			Name:  "cat_combined_ns",
			Args:  []string{"-n", "-s"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		// R3.3: -b -s combined.
		{
			Name:  "cat_combined_bs",
			Args:  []string{"-b", "-s"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		// R4.1, R4.2: show non-printing.
		{
			Name: "cat_show_nonprinting",
			Args: []string{"-v", nonPrintFile},
		},
		// R4.1: -v via stdin with binary.
		{
			Name:  "cat_show_nonprinting_stdin",
			Args:  []string{"-v"},
			Stdin: []byte{0x01, 0x09, 0x1b, 0x7f, 0x80, 0xff},
		},
		// R4.3: show ends.
		{
			Name:  "cat_show_ends",
			Args:  []string{"-E"},
			Stdin: []byte("line one\nline two\n"),
		},
		// R4.4: show tabs.
		{
			Name: "cat_show_tabs",
			Args: []string{"-T", tabFile},
		},
		// R4.5: -A = -vET.
		{
			Name: "cat_show_all",
			Args: []string{"-A", showAllFile},
		},
		// R4.6: -e = -vE.
		{
			Name: "cat_flag_e",
			Args: []string{"-e", eFile},
		},
		// R4.7: -t = -vT.
		{
			Name: "cat_flag_t",
			Args: []string{"-t", tFile},
		},
		// R4.8: -u accepted.
		{
			Name:  "cat_flag_u_accepted",
			Args:  []string{"-u"},
			Stdin: []byte("test\n"),
		},
		// R5.2: missing file.
		{
			Name:     "cat_missing_file",
			Args:     []string{nonexistent, realFile},
			ExitCode: 1,
		},
		// R3.2: squeeze across file boundaries.
		{
			Name: "cat_squeeze_across_files",
			Args: []string{"-s",
				testutils.WriteTestFile(t, dir, "trail_blank.txt", "x\n\n"),
				testutils.WriteTestFile(t, dir, "lead_blank.txt", "\ny\n"),
			},
		},
		// Combined: -n -v -E -T (all transformations).
		{
			Name: "cat_all_flags_combined",
			Args: []string{"-n", "-v", "-E", "-T", showAllFile},
		},
		// -A on binary input with all byte values.
		{
			Name: "cat_show_all_binary",
			Args: []string{"-A", binFile},
		},
		// -b -s -v combined.
		{
			Name:  "cat_bsv_combined",
			Args:  []string{"-b", "-s", "-v"},
			Stdin: []byte("hello\n\n\n\x01world\n"),
		},
		// Multiple files with -n (line numbers continue across files).
		{
			Name: "cat_n_across_files",
			Args: []string{"-n", file2, file3},
		},
		// Empty stdin with -n.
		{
			Name:  "cat_empty_stdin_n",
			Args:  []string{"-n"},
			Stdin: []byte{},
		},
		// Single newline.
		{
			Name:  "cat_single_newline",
			Args:  []string{"-n"},
			Stdin: []byte("\n"),
		},
		// No trailing newline.
		{
			Name:  "cat_no_trailing_newline",
			Args:  []string{"-n"},
			Stdin: []byte("no newline"),
		},
		// -E on input without trailing newline.
		{
			Name:  "cat_E_no_trailing_newline",
			Args:  []string{"-E"},
			Stdin: []byte("no newline"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiff_EmptyFile tests cat with an empty file.
func TestDiff_EmptyFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcat")
	if err != nil {
		t.Skipf("reference binary gcat not in PATH: %v", err)
	}

	dir := t.TempDir()
	emptyFile := testutils.WriteTestFile(t, dir, "empty.txt", "")

	tests := []testutils.DiffTest{
		{
			Name: "cat_empty_file",
			Args: []string{emptyFile},
		},
		{
			Name: "cat_empty_file_n",
			Args: []string{"-n", emptyFile},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestBuild verifies that the binary compiles without errors (AC1).
func TestBuild(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("go", "build", "-o", os.DevNull, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build failed: %v", err)
	}
}
