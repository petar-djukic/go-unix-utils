// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/comm against gcomm (GNU coreutils).
// Covers prd029-comm R1.1-R1.4: three-column comparison output,
// stdin via '-', column suppression flags (-1, -2, -3), and
// --version/--help exit behavior.
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
