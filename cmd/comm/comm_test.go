// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/comm against gcomm (GNU coreutils).
// Covers prd029-comm R1.1-R1.4: three-column comparison output,
// stdin via '-', column suppression flags (-1, -2, -3), and
// --version/--help exit behavior.
// Covers prd029-comm R2.1-R2.4: --check-order, --nocheck-order,
// and unsorted input handling.
// Covers prd029-comm R3.1-R3.4: output delimiter and column suppression options.
// Covers prd029-comm R4.1-R4.4: --total summary line and --zero-terminated mode.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgNameNormalizer replaces the reference binary name (gcomm or its
// full path) with the Go binary name (comm) in stderr so error message
// comparisons match.
func stderrProgNameNormalizer(data []byte) []byte {
	for {
		pos := bytes.Index(data, []byte("gcomm"))
		if pos < 0 {
			break
		}
		start := pos
		for start > 0 && data[start-1] != '\'' && data[start-1] != '"' && data[start-1] != ' ' && data[start-1] != '\n' {
			start--
		}
		data = append(data[:start], append([]byte("comm"), data[pos+len("gcomm"):]...)...)
	}
	return data
}

// writeTestFile creates a file with the given content in dir and returns
// its path.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return p
}

// TestDiff tests R1.1-R1.2: basic three-column output for two sorted files.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	dir := t.TempDir()
	file1 := writeTestFile(t, dir, "f1.txt", "a\nb\nc\n")
	file2 := writeTestFile(t, dir, "f2.txt", "b\nc\nd\n")
	file3 := writeTestFile(t, dir, "f3.txt", "a\nb\nc\n")
	file4 := writeTestFile(t, dir, "f4.txt", "a\nb\nc\n")
	empty := writeTestFile(t, dir, "empty.txt", "")
	single := writeTestFile(t, dir, "single.txt", "hello\n")
	allDiff1 := writeTestFile(t, dir, "alldiff1.txt", "a\nc\ne\n")
	allDiff2 := writeTestFile(t, dir, "alldiff2.txt", "b\nd\nf\n")

	tests := []testutils.DiffTest{
		// R1.1: basic three-column output — a unique to f1, d unique to f2, b and c common.
		{
			Name:    "basic_three_column",
			Args:    []string{file1, file2},
			WorkDir: dir,
		},
		// R1.1: identical files — all lines in column 3.
		{
			Name:    "identical_files",
			Args:    []string{file3, file4},
			WorkDir: dir,
		},
		// R1.3: one file empty — all lines from other in column 1.
		{
			Name:    "file1_empty",
			Args:    []string{empty, file1},
			WorkDir: dir,
		},
		// R1.3: other file empty — all lines in column 2.
		{
			Name:    "file2_empty",
			Args:    []string{file1, empty},
			WorkDir: dir,
		},
		// R1.1: both files empty.
		{
			Name:    "both_empty",
			Args:    []string{empty, empty},
			WorkDir: dir,
		},
		// R1.1: single-line files.
		{
			Name:    "single_line_files",
			Args:    []string{single, single},
			WorkDir: dir,
		},
		// R1.1: completely disjoint files — interleaved columns 1 and 2.
		{
			Name:    "disjoint_files",
			Args:    []string{allDiff1, allDiff2},
			WorkDir: dir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffColumnSuppression tests R1.3: -1, -2, -3 column suppression flags.
func TestDiffColumnSuppression(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	dir := t.TempDir()
	file1 := writeTestFile(t, dir, "f1.txt", "a\nb\nc\n")
	file2 := writeTestFile(t, dir, "f2.txt", "b\nc\nd\n")

	tests := []testutils.DiffTest{
		// R1.3: -1 suppresses column 1 (lines unique to file1).
		{
			Name:    "suppress_col1",
			Args:    []string{"-1", file1, file2},
			WorkDir: dir,
		},
		// R1.3: -2 suppresses column 2 (lines unique to file2).
		{
			Name:    "suppress_col2",
			Args:    []string{"-2", file1, file2},
			WorkDir: dir,
		},
		// R1.3: -3 suppresses column 3 (common lines).
		{
			Name:    "suppress_col3",
			Args:    []string{"-3", file1, file2},
			WorkDir: dir,
		},
		// R1.3: -12 suppresses columns 1 and 2 — only common lines remain.
		{
			Name:    "suppress_col12",
			Args:    []string{"-12", file1, file2},
			WorkDir: dir,
		},
		// R1.3: -13 suppresses columns 1 and 3 — only file2-unique lines.
		{
			Name:    "suppress_col13",
			Args:    []string{"-13", file1, file2},
			WorkDir: dir,
		},
		// R1.3: -23 suppresses columns 2 and 3 — only file1-unique lines.
		{
			Name:    "suppress_col23",
			Args:    []string{"-23", file1, file2},
			WorkDir: dir,
		},
		// R1.3: -123 suppresses all columns — no output.
		{
			Name:    "suppress_all",
			Args:    []string{"-123", file1, file2},
			WorkDir: dir,
		},
		// R1.3: separate flags -1 -2 -3.
		{
			Name:    "suppress_all_separate",
			Args:    []string{"-1", "-2", "-3", file1, file2},
			WorkDir: dir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffStdin tests R1.2: accept '-' as a filename to read from stdin.
func TestDiffStdin(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	dir := t.TempDir()
	file1 := writeTestFile(t, dir, "f1.txt", "a\nb\nc\n")
	file2 := writeTestFile(t, dir, "f2.txt", "b\nc\nd\n")

	tests := []testutils.DiffTest{
		// R1.2: stdin as file1.
		{
			Name:    "stdin_as_file1",
			Args:    []string{"-", file2},
			Stdin:   []byte("a\nb\nc\n"),
			WorkDir: dir,
		},
		// R1.2: stdin as file2.
		{
			Name:    "stdin_as_file2",
			Args:    []string{file1, "-"},
			Stdin:   []byte("b\nc\nd\n"),
			WorkDir: dir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffVersionHelp tests R1.4: --version and --help flags.
func TestDiffVersionHelp(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// --version: exits 0 and produces output.
	t.Run("version", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(goBin, "--version")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("--version failed: %v", err)
		}
		if len(out) == 0 {
			t.Error("--version produced no output")
		}
	})

	// --help: exits 0 and produces output.
	t.Run("help", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(goBin, "--help")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("--help failed: %v", err)
		}
		if len(out) == 0 {
			t.Error("--help produced no output")
		}
	})
}

// TestDiffMissingOperand tests R1.4: exit non-zero when fewer than two files given.
func TestDiffMissingOperand(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:      "no_args",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFileNotFound tests R1.4: exit non-zero when a file cannot be opened.
func TestDiffFileNotFound(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	dir := t.TempDir()
	file1 := writeTestFile(t, dir, "f1.txt", "a\n")

	tests := []testutils.DiffTest{
		{
			Name:      "file1_not_found",
			Args:      []string{"/nonexistent/path.txt", file1},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		{
			Name:      "file2_not_found",
			Args:      []string{file1, "/nonexistent/path.txt"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffOrderChecking tests R2.1-R2.4: --check-order, --nocheck-order,
// and default behavior with unsorted input.
func TestDiffOrderChecking(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	dir := t.TempDir()
	sorted1 := writeTestFile(t, dir, "sorted1.txt", "a\nb\nc\n")
	sorted2 := writeTestFile(t, dir, "sorted2.txt", "b\nc\nd\n")
	unsorted1 := writeTestFile(t, dir, "unsorted1.txt", "b\na\nc\n")
	unsorted2 := writeTestFile(t, dir, "unsorted2.txt", "c\na\nd\n")

	tests := []testutils.DiffTest{
		// R2.1: --check-order with unsorted file1 prints diagnostic to stderr.
		{
			Name:      "check_order_unsorted_file1",
			Args:      []string{"--check-order", unsorted1, sorted2},
			WorkDir:   dir,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R2.1: --check-order with unsorted file2 prints diagnostic to stderr.
		{
			Name:      "check_order_unsorted_file2",
			Args:      []string{"--check-order", sorted1, unsorted2},
			WorkDir:   dir,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R2.1: --check-order with sorted input produces no diagnostic.
		{
			Name:    "check_order_sorted_input",
			Args:    []string{"--check-order", sorted1, sorted2},
			WorkDir: dir,
		},
		// R2.2: --nocheck-order with unsorted input produces no diagnostic.
		{
			Name:      "nocheck_order_unsorted",
			Args:      []string{"--nocheck-order", unsorted1, unsorted2},
			WorkDir:   dir,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R2.3: default (no flag) with unsorted input — warns and exits 1.
		{
			Name:      "default_unsorted",
			Args:      []string{unsorted1, unsorted2},
			WorkDir:   dir,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// D2: last flag wins — --nocheck-order after --check-order.
		{
			Name:      "last_flag_wins_nocheck",
			Args:      []string{"--check-order", "--nocheck-order", unsorted1, sorted2},
			WorkDir:   dir,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// D2: last flag wins — --check-order after --nocheck-order.
		{
			Name:      "last_flag_wins_check",
			Args:      []string{"--nocheck-order", "--check-order", unsorted1, sorted2},
			WorkDir:   dir,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R2.1: --check-order combined with column suppression.
		{
			Name:      "check_order_with_suppress",
			Args:      []string{"--check-order", "-1", unsorted1, sorted2},
			WorkDir:   dir,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffOutputDelimiter tests R3.4: --output-delimiter=STRING replaces
// tab as the column separator.
func TestDiffOutputDelimiter(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	dir := t.TempDir()
	file1 := writeTestFile(t, dir, "f1.txt", "a\nb\nc\n")
	file2 := writeTestFile(t, dir, "f2.txt", "b\nc\nd\n")

	tests := []testutils.DiffTest{
		// R3.4: --output-delimiter=| uses | instead of tab.
		{
			Name:    "output_delimiter_pipe",
			Args:    []string{"--output-delimiter=|", file1, file2},
			WorkDir: dir,
		},
		// R3.4: --output-delimiter with multi-character string.
		{
			Name:    "output_delimiter_multi_char",
			Args:    []string{"--output-delimiter=<=>", file1, file2},
			WorkDir: dir,
		},
		// R3.4: --output-delimiter combined with column suppression -1.
		{
			Name:    "output_delimiter_with_suppress1",
			Args:    []string{"--output-delimiter=,", "-1", file1, file2},
			WorkDir: dir,
		},
		// R3.4: --output-delimiter combined with column suppression -12.
		{
			Name:    "output_delimiter_with_suppress12",
			Args:    []string{"--output-delimiter=,", "-12", file1, file2},
			WorkDir: dir,
		},
		// R3.4: --output-delimiter combined with column suppression -3.
		{
			Name:    "output_delimiter_with_suppress3",
			Args:    []string{"--output-delimiter=,", "-3", file1, file2},
			WorkDir: dir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffTotal tests R4.1 and R4.3: --total appends a summary line with
// column counts, working correctly with column suppression flags.
func TestDiffTotal(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	dir := t.TempDir()
	file1 := writeTestFile(t, dir, "f1.txt", "a\nb\nc\n")
	file2 := writeTestFile(t, dir, "f2.txt", "b\nc\nd\n")
	empty := writeTestFile(t, dir, "empty.txt", "")
	identical1 := writeTestFile(t, dir, "id1.txt", "a\nb\nc\n")
	identical2 := writeTestFile(t, dir, "id2.txt", "a\nb\nc\n")

	tests := []testutils.DiffTest{
		// R4.1: --total with basic three-column output.
		{
			Name:    "total_basic",
			Args:    []string{"--total", file1, file2},
			WorkDir: dir,
		},
		// R4.3: --total with -1 suppression.
		{
			Name:    "total_suppress1",
			Args:    []string{"--total", "-1", file1, file2},
			WorkDir: dir,
		},
		// R4.3: --total with -2 -3 suppression.
		{
			Name:    "total_suppress23",
			Args:    []string{"--total", "-23", file1, file2},
			WorkDir: dir,
		},
		// R4.3: --total with all columns suppressed.
		{
			Name:    "total_suppress_all",
			Args:    []string{"--total", "-123", file1, file2},
			WorkDir: dir,
		},
		// R4.1: --total with identical files (all in column 3).
		{
			Name:    "total_identical",
			Args:    []string{"--total", identical1, identical2},
			WorkDir: dir,
		},
		// R4.1: --total with empty files.
		{
			Name:    "total_both_empty",
			Args:    []string{"--total", empty, empty},
			WorkDir: dir,
		},
		// R4.1: --total combined with --output-delimiter.
		{
			Name:    "total_with_output_delimiter",
			Args:    []string{"--total", "--output-delimiter=|", file1, file2},
			WorkDir: dir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffZeroTerminated tests R4.2 and R4.4: -z/--zero-terminated uses NUL
// as the line delimiter for both input and output, combined with other flags.
func TestDiffZeroTerminated(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	dir := t.TempDir()
	// NUL-delimited input files.
	zFile1 := filepath.Join(dir, "z1.txt")
	if err := os.WriteFile(zFile1, []byte("a\x00b\x00c\x00"), 0o644); err != nil {
		t.Fatalf("writing z1.txt: %v", err)
	}
	zFile2 := filepath.Join(dir, "z2.txt")
	if err := os.WriteFile(zFile2, []byte("b\x00c\x00d\x00"), 0o644); err != nil {
		t.Fatalf("writing z2.txt: %v", err)
	}
	zEmpty := filepath.Join(dir, "zempty.txt")
	if err := os.WriteFile(zEmpty, []byte(""), 0o644); err != nil {
		t.Fatalf("writing zempty.txt: %v", err)
	}
	zIdentical := filepath.Join(dir, "zid.txt")
	if err := os.WriteFile(zIdentical, []byte("a\x00b\x00c\x00"), 0o644); err != nil {
		t.Fatalf("writing zid.txt: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.2: basic -z three-column output.
		{
			Name:    "zero_terminated_basic",
			Args:    []string{"-z", zFile1, zFile2},
			WorkDir: dir,
		},
		// R4.2: --zero-terminated long form.
		{
			Name:    "zero_terminated_long",
			Args:    []string{"--zero-terminated", zFile1, zFile2},
			WorkDir: dir,
		},
		// R4.4: -z combined with column suppression -12.
		{
			Name:    "zero_terminated_suppress12",
			Args:    []string{"-z", "-12", zFile1, zFile2},
			WorkDir: dir,
		},
		// R4.4: -z combined with --output-delimiter.
		{
			Name:    "zero_terminated_output_delimiter",
			Args:    []string{"-z", "--output-delimiter=|", zFile1, zFile2},
			WorkDir: dir,
		},
		// R4.4: -z combined with --total.
		{
			Name:    "zero_terminated_total",
			Args:    []string{"-z", "--total", zFile1, zFile2},
			WorkDir: dir,
		},
		// R4.4: -z combined with --total and column suppression.
		{
			Name:    "zero_terminated_total_suppress1",
			Args:    []string{"-z", "--total", "-1", zFile1, zFile2},
			WorkDir: dir,
		},
		// R4.2: -z with empty files.
		{
			Name:    "zero_terminated_empty",
			Args:    []string{"-z", zEmpty, zEmpty},
			WorkDir: dir,
		},
		// R4.2: -z with identical files.
		{
			Name:    "zero_terminated_identical",
			Args:    []string{"-z", zFile1, zIdentical},
			WorkDir: dir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
