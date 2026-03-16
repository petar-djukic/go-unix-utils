// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/uniq against guniq (GNU coreutils).
// Covers prd028-uniq R1.1-R1.4: default adjacent-line deduplication,
// stdin/file input, output file, and exit code behavior.
// Covers prd028-uniq R2.1-R2.4: counting (-c), duplicate filtering (-d, -u),
// and all-repeated (-D) output modes.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgNameNormalizer replaces the reference binary name (guniq or its
// full path) with the Go binary name (uniq) in stderr so error message
// comparisons match.
func stderrProgNameNormalizer(data []byte) []byte {
	// Replace full-path occurrences first (e.g., "/opt/homebrew/bin/guniq:" → "uniq:").
	for {
		idx := bytes.Index(data, []byte("/"))
		if idx < 0 {
			break
		}
		end := bytes.Index(data[idx:], []byte("guniq:"))
		if end < 0 {
			break
		}
		data = append(data[:idx], append([]byte("uniq:"), data[idx+end+len("guniq:"):]...)...)
	}
	// Replace bare guniq: occurrences.
	data = bytes.ReplaceAll(data, []byte("guniq:"), []byte("uniq:"))
	return data
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: suppress adjacent duplicate lines.
		{
			Name:  "adjacent_duplicates",
			Stdin: []byte("a\na\nb\na\n"),
		},
		// R1.1: all lines identical.
		{
			Name:  "all_identical",
			Stdin: []byte("x\nx\nx\nx\n"),
		},
		// R1.2: no duplicates — all lines unique.
		{
			Name:  "no_duplicates",
			Stdin: []byte("a\nb\nc\nd\n"),
		},
		// R1.2: single line input.
		{
			Name:  "single_line",
			Stdin: []byte("hello\n"),
		},
		// R1.1: empty input produces no output.
		{
			Name:  "empty_input",
			Stdin: []byte(""),
		},
		// R1.4: case-sensitive comparison — 'A' and 'a' are different.
		{
			Name:  "case_sensitive",
			Stdin: []byte("A\na\nA\n"),
		},
		// R1.1: multiple runs of duplicates.
		{
			Name:  "multiple_runs",
			Stdin: []byte("a\na\nb\nb\nc\nc\n"),
		},
		// R1.1: line without trailing newline at end of input.
		{
			Name:  "no_trailing_newline",
			Stdin: []byte("a\na\nb"),
		},
		// R1.2: non-adjacent duplicates are not suppressed.
		{
			Name:  "non_adjacent_duplicates",
			Stdin: []byte("a\nb\na\nb\n"),
		},
		// R1.1: blank lines are treated as identical adjacent lines.
		{
			Name:  "blank_lines",
			Stdin: []byte("\n\na\n\n\n"),
		},
		// R1.1: lines with leading/trailing spaces are compared exactly.
		{
			Name:  "whitespace_significant",
			Stdin: []byte("a \na\n a\n"),
		},
		// R1.3: read from stdin when '-' is given explicitly.
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("x\nx\ny\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCount tests R2.4: -c prefixes lines with occurrence count.
func TestDiffCount(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R2.4: count with mixed duplicates.
		{
			Name:  "count_mixed",
			Args:  []string{"-c"},
			Stdin: []byte("a\na\nb\na\n"),
		},
		// R2.4: count with single lines.
		{
			Name:  "count_single_lines",
			Args:  []string{"-c"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R2.4: count with all identical.
		{
			Name:  "count_all_identical",
			Args:  []string{"-c"},
			Stdin: []byte("x\nx\nx\nx\n"),
		},
		// R2.4: count with no trailing newline.
		{
			Name:  "count_no_trailing_newline",
			Args:  []string{"-c"},
			Stdin: []byte("a\na\nb"),
		},
		// R2.4: count with empty input.
		{
			Name:  "count_empty",
			Args:  []string{"-c"},
			Stdin: []byte(""),
		},
		// R2.4: --count long form.
		{
			Name:  "count_long_flag",
			Args:  []string{"--count"},
			Stdin: []byte("a\na\nb\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffRepeated tests R2.1: -d prints only duplicate lines.
func TestDiffRepeated(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R2.1: only duplicated lines survive.
		{
			Name:  "repeated_mixed",
			Args:  []string{"-d"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R2.1: no duplicates — empty output.
		{
			Name:  "repeated_no_dups",
			Args:  []string{"-d"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R2.1: all identical — one output line.
		{
			Name:  "repeated_all_identical",
			Args:  []string{"-d"},
			Stdin: []byte("x\nx\nx\n"),
		},
		// R2.1: --repeated long form.
		{
			Name:  "repeated_long_flag",
			Args:  []string{"--repeated"},
			Stdin: []byte("a\na\nb\n"),
		},
		// R2.1: single line — not repeated, empty output.
		{
			Name:  "repeated_single_line",
			Args:  []string{"-d"},
			Stdin: []byte("hello\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffUnique tests R2.3: -u prints only unique (non-repeated) lines.
func TestDiffUnique(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R2.3: only unique lines survive.
		{
			Name:  "unique_mixed",
			Args:  []string{"-u"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R2.3: no duplicates — all lines output.
		{
			Name:  "unique_all_unique",
			Args:  []string{"-u"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R2.3: all identical — empty output.
		{
			Name:  "unique_all_identical",
			Args:  []string{"-u"},
			Stdin: []byte("x\nx\nx\n"),
		},
		// R2.3: --unique long form.
		{
			Name:  "unique_long_flag",
			Args:  []string{"--unique"},
			Stdin: []byte("a\na\nb\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffAllRepeated tests R2.2/R2.4: -D prints all duplicate lines with
// optional delimiter methods (none, prepend, separate).
func TestDiffAllRepeated(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R2.2: -D prints all lines of duplicate groups.
		{
			Name:  "all_repeated_default",
			Args:  []string{"-D"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R2.4: --all-repeated=none (same as -D).
		{
			Name:  "all_repeated_none",
			Args:  []string{"--all-repeated=none"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R2.4: --all-repeated=prepend adds blank line before each group.
		{
			Name:  "all_repeated_prepend",
			Args:  []string{"--all-repeated=prepend"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R2.4: --all-repeated=separate adds blank line between groups.
		{
			Name:  "all_repeated_separate",
			Args:  []string{"--all-repeated=separate"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R2.2: no duplicates — empty output for -D.
		{
			Name:  "all_repeated_no_dups",
			Args:  []string{"-D"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R2.2: all identical — all lines output.
		{
			Name:  "all_repeated_all_identical",
			Args:  []string{"-D"},
			Stdin: []byte("x\nx\nx\n"),
		},
		// R2.4: --all-repeated bare (no =METHOD) defaults to none.
		{
			Name:  "all_repeated_bare_long",
			Args:  []string{"--all-repeated"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R2.4: prepend with single duplicate group.
		{
			Name:  "all_repeated_prepend_single_group",
			Args:  []string{"--all-repeated=prepend"},
			Stdin: []byte("a\nb\nb\nc\n"),
		},
		// R2.4: separate with multiple groups.
		{
			Name:  "all_repeated_separate_multi",
			Args:  []string{"--all-repeated=separate"},
			Stdin: []byte("a\na\nb\nc\nc\nd\nd\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCombinedFlags tests AC5: combined flag interactions (-c -d, -c -u).
func TestDiffCombinedFlags(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// AC5: -c -d counts only repeated groups.
		{
			Name:  "count_repeated",
			Args:  []string{"-c", "-d"},
			Stdin: []byte("a\na\nb\nc\nc\nc\n"),
		},
		// AC5: -c -u counts only unique groups.
		{
			Name:  "count_unique",
			Args:  []string{"-c", "-u"},
			Stdin: []byte("a\na\nb\nc\nc\nc\n"),
		},
		// AC5: -cd combined short form.
		{
			Name:  "count_repeated_short",
			Args:  []string{"-cd"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// AC5: -cu combined short form.
		{
			Name:  "count_unique_short",
			Args:  []string{"-cu"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestInputFile tests R1.2: reading from a named input file.
func TestInputFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	dir := t.TempDir()
	inputFile := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(inputFile, []byte("a\na\nb\nc\nc\n"), 0o644); err != nil {
		t.Fatalf("writing input file: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:    "read_from_file",
			Args:    []string{inputFile},
			WorkDir: dir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestOutputFile tests R1.3: writing to a named output file.
func TestOutputFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	// Create input file in a shared location, use separate dirs for output.
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(inputFile, []byte("a\na\nb\nb\nc\n"), 0o644); err != nil {
		t.Fatalf("writing input file: %v", err)
	}

	goOutDir := filepath.Join(dir, "go_out")
	refOutDir := filepath.Join(dir, "ref_out")
	if err := os.Mkdir(goOutDir, 0o755); err != nil {
		t.Fatalf("creating go output dir: %v", err)
	}
	if err := os.Mkdir(refOutDir, 0o755); err != nil {
		t.Fatalf("creating ref output dir: %v", err)
	}

	goOutput := filepath.Join(goOutDir, "out.txt")
	refOutput := filepath.Join(refOutDir, "out.txt")

	// Run Go binary.
	goCmd := exec.Command(goBin, inputFile, goOutput)
	goCmd.Env = append(os.Environ(), "LC_ALL=C")
	if out, err := goCmd.CombinedOutput(); err != nil {
		t.Fatalf("go binary failed: %v\n%s", err, out)
	}

	// Run reference binary.
	refCmd := exec.Command(refBin, inputFile, refOutput)
	refCmd.Env = append(os.Environ(), "LC_ALL=C")
	if out, err := refCmd.CombinedOutput(); err != nil {
		t.Fatalf("ref binary failed: %v\n%s", err, out)
	}

	// Compare output files.
	goData, err := os.ReadFile(goOutput)
	if err != nil {
		t.Fatalf("reading go output: %v", err)
	}
	refData, err := os.ReadFile(refOutput)
	if err != nil {
		t.Fatalf("reading ref output: %v", err)
	}

	if string(goData) != string(refData) {
		t.Errorf("output file mismatch:\ngo:  %q\nref: %q", goData, refData)
	}
}

// TestFileNotFound tests R1.4: exit non-zero when input file does not exist.
func TestFileNotFound(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:      "nonexistent_file",
			Args:      []string{"/nonexistent/path/to/file.txt"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
