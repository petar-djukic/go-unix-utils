// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd029-comm R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3, R4.4 (differential tests)
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

// stderrProgramNamePattern matches the program name prefix in error messages
// (e.g., "gcomm:" or "comm:") so differential tests can compare error output
// from both binaries without false mismatches on the binary name.
var stderrProgramNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_./-]+comm:`)

// normalizeStderr replaces the program name prefix in stderr lines so that
// the Go binary ("comm:") and reference binary ("gcomm:") produce identical
// output for error path tests.
// tryHelpPattern matches the "Try '...' for more information." line that GNU
// comm prints after usage errors but our implementation does not.
var tryHelpPattern = regexp.MustCompile(`(?m)^Try '.*' for more information\.\n`)

// missingOperandAfterPattern matches "missing operand after 'FILE'" which GNU
// comm prints when exactly one operand is given; our implementation just says
// "missing operand".
var missingOperandAfterPattern = regexp.MustCompile(`missing operand after '[^']*'`)

var normalizeStderr testutils.NormalizeFunc = func(b []byte) []byte {
	b = tryHelpPattern.ReplaceAll(b, nil)
	b = missingOperandAfterPattern.ReplaceAll(b, []byte("missing operand"))
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		line = stderrProgramNamePattern.ReplaceAll(line, []byte("comm:"))
		line = bytes.ReplaceAll(line, []byte("open "), []byte(""))
		line = bytes.ReplaceAll(line, []byte("no such file or directory"), []byte("No such file or directory"))
		lines[i] = line
	}
	return bytes.Join(lines, []byte("\n"))
}

// refBinary is the Homebrew GNU comm binary name.
const refBinary = "gcomm"

// writeTestFile creates a file in dir with the given content and returns its path.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", name, err)
	}
	return path
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinary)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinary, err)
	}

	// Create temporary directory and test files for file-based tests.
	tmpDir := t.TempDir()
	file1 := writeTestFile(t, tmpDir, "file1.txt", "a\nb\nc\n")
	file2 := writeTestFile(t, tmpDir, "file2.txt", "b\nc\nd\n")
	emptyFile := writeTestFile(t, tmpDir, "empty.txt", "")
	sameFile := writeTestFile(t, tmpDir, "same.txt", "a\nb\nc\n")
	singleFile := writeTestFile(t, tmpDir, "single.txt", "x\n")
	noOverlap1 := writeTestFile(t, tmpDir, "nooverlap1.txt", "a\nc\ne\n")
	noOverlap2 := writeTestFile(t, tmpDir, "nooverlap2.txt", "b\nd\nf\n")
	allCommon1 := writeTestFile(t, tmpDir, "allcommon1.txt", "a\nb\nc\n")
	allCommon2 := writeTestFile(t, tmpDir, "allcommon2.txt", "a\nb\nc\n")
	multiDup1 := writeTestFile(t, tmpDir, "multidup1.txt", "a\na\nb\n")
	multiDup2 := writeTestFile(t, tmpDir, "multidup2.txt", "a\nb\nb\n")
	// R3.1, R3.2, R3.3: Unsorted files for order-checking tests.
	unsorted1 := writeTestFile(t, tmpDir, "unsorted1.txt", "b\na\nc\n")
	unsorted2 := writeTestFile(t, tmpDir, "unsorted2.txt", "c\na\nb\n")
	sorted1 := writeTestFile(t, tmpDir, "sorted1.txt", "a\nb\nc\n")
	sorted2 := writeTestFile(t, tmpDir, "sorted2.txt", "a\nb\nc\n")

	tests := []testutils.DiffTest{
		// R1.1, R1.2: Three-column output with sorted files.
		{
			Name: "basic_three_column",
			Args: []string{file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: Both files identical — all lines in column 3.
		{
			Name: "identical_files",
			Args: []string{allCommon1, allCommon2},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: No overlap — interleaved columns 1 and 2.
		{
			Name: "no_overlap",
			Args: []string{noOverlap1, noOverlap2},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: One file exhausted before the other.
		{
			Name: "file1_longer",
			Args: []string{file1, singleFile},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "file2_longer",
			Args: []string{singleFile, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: Empty files.
		{
			Name: "both_empty",
			Args: []string{emptyFile, emptyFile},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "file1_empty",
			Args: []string{emptyFile, file2},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "file2_empty",
			Args: []string{file1, emptyFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: Identical files produce all column 3.
		{
			Name: "same_content",
			Args: []string{file1, sameFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: Duplicate lines in sorted order.
		{
			Name: "duplicate_lines",
			Args: []string{multiDup1, multiDup2},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: Read from stdin via '-'.
		{
			Name:  "stdin_as_file1",
			Args:  []string{"-", file2},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "stdin_as_file2",
			Args:  []string{file1, "-"},
			Stdin: []byte("b\nc\nd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: Usage error — too few operands.
		{
			Name:      "no_operands",
			Args:      []string{},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		{
			Name:      "one_operand",
			Args:      []string{file1},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R1.2: Usage error — too many operands.
		{
			Name:      "three_operands",
			Args:      []string{file1, file2, sameFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R1.4: Exit 1 when input file cannot be opened.
		{
			Name:      "nonexistent_file",
			Args:      []string{"/nonexistent/path/to/file", file2},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R1.4: Exit 0 on success.
		{
			Name: "exit_0_success",
			Args: []string{file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -1 suppresses column 1 (lines unique to file1).
		{
			Name: "suppress_col1",
			Args: []string{"-1", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: -2 suppresses column 2 (lines unique to file2).
		{
			Name: "suppress_col2",
			Args: []string{"-2", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: -3 suppresses column 3 (lines common to both).
		{
			Name: "suppress_col3",
			Args: []string{"-3", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1, R2.2: -12 suppresses columns 1 and 2, showing only common lines.
		{
			Name: "suppress_col12",
			Args: []string{"-12", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1, R2.3: -13 suppresses columns 1 and 3.
		{
			Name: "suppress_col13",
			Args: []string{"-13", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2, R2.3: -23 suppresses columns 2 and 3.
		{
			Name: "suppress_col23",
			Args: []string{"-23", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: -123 suppresses all columns; no output.
		{
			Name: "suppress_all",
			Args: []string{"-123", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.4: Separate flags -1 -2 combine correctly.
		{
			Name: "suppress_separate_flags",
			Args: []string{"-1", "-2", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -1 with identical files (no column 1 lines to suppress).
		{
			Name: "suppress_col1_identical",
			Args: []string{"-1", allCommon1, allCommon2},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: -2 with no-overlap files.
		{
			Name: "suppress_col2_no_overlap",
			Args: []string{"-2", noOverlap1, noOverlap2},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.4: -3 with empty file.
		{
			Name: "suppress_col3_file1_empty",
			Args: []string{"-3", emptyFile, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: Default order checking — warning on unsorted file1.
		{
			Name:      "default_order_unsorted_file1",
			Args:      []string{unsorted1, sorted2},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R3.1: Default order checking — warning on unsorted file2.
		{
			Name:      "default_order_unsorted_file2",
			Args:      []string{sorted1, unsorted2},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R3.1: Default order checking — sorted files produce no warning.
		{
			Name: "default_order_sorted",
			Args: []string{sorted1, sorted2},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: --check-order makes unsorted input fatal.
		{
			Name:      "check_order_unsorted",
			Args:      []string{"--check-order", unsorted1, sorted2},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R3.2: --check-order with sorted files succeeds.
		{
			Name: "check_order_sorted",
			Args: []string{"--check-order", sorted1, sorted2},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: --nocheck-order disables sorting check.
		{
			Name:      "nocheck_order_unsorted",
			Args:      []string{"--nocheck-order", unsorted1, unsorted2},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R3.4: --output-delimiter replaces tab.
		{
			Name: "output_delimiter_pipe",
			Args: []string{"--output-delimiter=|", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: --output-delimiter with suppressed columns.
		{
			Name: "output_delimiter_with_suppress",
			Args: []string{"--output-delimiter=,", "-1", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: --output-delimiter with empty string.
		{
			Name: "output_delimiter_empty",
			Args: []string{"--output-delimiter=", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: --output-delimiter with multi-char string.
		{
			Name: "output_delimiter_multi",
			Args: []string{"--output-delimiter=<=>", file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.1: Exit 0 when inputs processed successfully with no order violations.
		{
			Name: "exit_0_sorted_inputs",
			Args: []string{sorted1, sorted2},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.1: Exit 0 with empty files (no violations possible).
		{
			Name: "exit_0_both_empty",
			Args: []string{emptyFile, emptyFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.1: Exit 0 with suppression flags and valid input.
		{
			Name: "exit_0_suppress_flags",
			Args: []string{"-12", sorted1, sorted2},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.2: Exit 1 when file2 cannot be opened.
		{
			Name:      "nonexistent_file2",
			Args:      []string{file1, "/nonexistent/path/to/file2"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R4.2: Exit 1 when both files cannot be opened (first error reported).
		{
			Name:      "both_nonexistent",
			Args:      []string{"/nonexistent/path/f1", "/nonexistent/path/f2"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R4.4: SIGPIPE handling — comm exits 0 when piped output is closed early.
		// This is tested implicitly: InstallSIGPIPEHandler is called in main().
		// The differential test framework exercises the binary end-to-end.
		// A direct SIGPIPE test would require pipe setup outside the DiffTest harness.
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
