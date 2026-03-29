// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/uniq implementing prd028-uniq R1.1-R1.4, R2.1-R2.4.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests against the guniq reference binary.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skip("reference binary guniq not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1: adjacent duplicates suppressed; non-adjacent kept.
		{
			Name:     "default_dedup",
			Args:     []string{},
			Stdin:    []byte("a\na\nb\na\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: single input line produces that line unchanged.
		{
			Name:     "single_line",
			Args:     []string{},
			Stdin:    []byte("only\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: empty input produces no output.
		{
			Name:     "empty_input",
			Args:     []string{},
			Stdin:    []byte{},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: all lines identical.
		{
			Name:     "all_identical",
			Args:     []string{},
			Stdin:    []byte("x\nx\nx\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.2: no adjacent duplicates — all lines pass through.
		{
			Name:     "no_duplicates",
			Args:     []string{},
			Stdin:    []byte("a\nb\nc\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.4: case-sensitive — 'A' and 'a' are different.
		{
			Name:     "case_sensitive",
			Args:     []string{},
			Stdin:    []byte("A\na\nA\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: multiple runs of duplicates.
		{
			Name:     "multiple_runs",
			Args:     []string{},
			Stdin:    []byte("a\na\nb\nb\nc\na\na\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.3: '-' reads stdin explicitly.
		{
			Name:     "dash_reads_stdin",
			Args:     []string{"-"},
			Stdin:    []byte("x\nx\ny\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: input without trailing newline.
		{
			Name:     "no_trailing_newline",
			Args:     []string{},
			Stdin:    []byte("a\na\nb"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1: -d outputs only lines with run length > 1 (one copy per run).
		{
			Name:     "duplicates_only",
			Args:     []string{"-d"},
			Stdin:    []byte("a\na\nb\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1: -d with no duplicates produces no output.
		{
			Name:     "duplicates_only_none",
			Args:     []string{"-d"},
			Stdin:    []byte("a\nb\nc\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1: -d with multiple duplicate runs.
		{
			Name:     "duplicates_only_multi",
			Args:     []string{"-d"},
			Stdin:    []byte("a\na\nb\nb\nc\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.2: -D outputs every line of duplicate runs.
		{
			Name:     "all_duplicates",
			Args:     []string{"-D"},
			Stdin:    []byte("a\na\nb\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.2: -D with no duplicates produces no output.
		{
			Name:     "all_duplicates_none",
			Args:     []string{"-D"},
			Stdin:    []byte("a\nb\nc\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.2: -D with multiple duplicate runs.
		{
			Name:     "all_duplicates_multi",
			Args:     []string{"-D"},
			Stdin:    []byte("a\na\na\nb\nb\nc\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.3: -u outputs only lines that appear exactly once.
		{
			Name:     "unique_only",
			Args:     []string{"-u"},
			Stdin:    []byte("a\na\nb\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.3: -u with all unique lines outputs everything.
		{
			Name:     "unique_only_all",
			Args:     []string{"-u"},
			Stdin:    []byte("a\nb\nc\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.3: -u with all duplicates produces no output.
		{
			Name:     "unique_only_none",
			Args:     []string{"-u"},
			Stdin:    []byte("a\na\nb\nb\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.4: -c prefixes each line with its run count.
		{
			Name:     "count",
			Args:     []string{"-c"},
			Stdin:    []byte("a\na\nb\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.4: -c with single occurrences.
		{
			Name:     "count_all_unique",
			Args:     []string{"-c"},
			Stdin:    []byte("a\nb\nc\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.4: -c with large run.
		{
			Name:     "count_large_run",
			Args:     []string{"-c"},
			Stdin:    []byte("x\nx\nx\nx\nx\ny\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1 + R2.4: -d -c combination.
		{
			Name:     "dup_count",
			Args:     []string{"-d", "-c"},
			Stdin:    []byte("a\na\nb\nc\nc\nc\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffInputFile tests reading from an input file (R1.2).
func TestDiffInputFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skip("reference binary guniq not in PATH")
	}

	dir := t.TempDir()
	inputFile := filepath.Join(dir, "input.txt")
	writeTestFile(t, inputFile, "a\na\nb\nc\nc\n")

	tests := []testutils.DiffTest{
		// R1.2: read from input file positional argument.
		{
			Name:     "input_file",
			Args:     []string{inputFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffOutputFile tests writing to an output file (R1.2).
func TestDiffOutputFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skip("reference binary guniq not in PATH")
	}

	dir := t.TempDir()
	inputFile := filepath.Join(dir, "input.txt")
	writeTestFile(t, inputFile, "a\na\nb\n")

	goOut := filepath.Join(dir, "go_output.txt")
	refOut := filepath.Join(dir, "ref_output.txt")

	// Run Go binary with output file.
	goCmd := exec.Command(goBin, inputFile, goOut)
	goCmd.Env = append(os.Environ(), "LC_ALL=C")
	if err := goCmd.Run(); err != nil {
		t.Fatalf("go binary failed: %v", err)
	}

	// Run reference binary with output file.
	refCmd := exec.Command(refBin, inputFile, refOut)
	refCmd.Env = append(os.Environ(), "LC_ALL=C")
	if err := refCmd.Run(); err != nil {
		t.Fatalf("ref binary failed: %v", err)
	}

	goContent, err := os.ReadFile(goOut)
	if err != nil {
		t.Fatalf("failed to read go output: %v", err)
	}
	refContent, err := os.ReadFile(refOut)
	if err != nil {
		t.Fatalf("failed to read ref output: %v", err)
	}
	if string(goContent) != string(refContent) {
		t.Errorf("output file mismatch:\ngo:  %q\nref: %q", goContent, refContent)
	}
}

// writeTestFile creates a file with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
}
