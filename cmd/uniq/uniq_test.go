// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/uniq against guniq (GNU coreutils).
// Covers prd028-uniq R1.1-R1.4: default adjacent-line deduplication,
// stdin/file input, output file, and exit code behavior.
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
