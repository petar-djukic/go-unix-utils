// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd053-sort R1.1, R1.2, R1.3, R1.4, R1.5, R1.6 differential tests
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsort")
	if err != nil {
		t.Skipf("reference binary gsort not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: Default lexicographic sort under LC_ALL=C.
		{
			Name:  "default_sort",
			Stdin: []byte("banana\napple\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Empty input.
		{
			Name:  "empty_input",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Single line.
		{
			Name:  "single_line",
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Already sorted input.
		{
			Name:  "already_sorted",
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Case sensitivity under LC_ALL=C (uppercase before lowercase).
		{
			Name:  "case_sensitive_sort",
			Stdin: []byte("b\nA\na\nB\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Lines with special characters.
		{
			Name:  "special_chars",
			Stdin: []byte("!\n@\n#\nz\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Duplicate lines.
		{
			Name:  "duplicate_lines",
			Stdin: []byte("b\na\nb\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: Read from stdin with "-" argument.
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("c\na\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: -r reverses sort order.
		{
			Name:  "reverse",
			Args:  []string{"-r"},
			Stdin: []byte("banana\napple\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: --reverse long form.
		{
			Name:  "reverse_long",
			Args:  []string{"--reverse"},
			Stdin: []byte("banana\napple\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.5: -u removes duplicate lines.
		{
			Name:  "unique",
			Args:  []string{"-u"},
			Stdin: []byte("b\na\nb\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.5: --unique long form.
		{
			Name:  "unique_long",
			Args:  []string{"--unique"},
			Stdin: []byte("c\nc\na\nb\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4 + R1.5: -ru combined.
		{
			Name:  "reverse_unique",
			Args:  []string{"-ru"},
			Stdin: []byte("b\na\nb\nc\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Lines with leading/trailing whitespace.
		{
			Name:  "whitespace_lines",
			Stdin: []byte(" b\na\n b\n a\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Numeric strings sorted lexicographically.
		{
			Name:  "numeric_lex_sort",
			Stdin: []byte("10\n2\n1\n20\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Empty lines sort first.
		{
			Name:  "empty_lines",
			Stdin: []byte("b\n\na\n\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.5: -u with all identical lines.
		{
			Name:  "unique_all_same",
			Args:  []string{"-u"},
			Stdin: []byte("a\na\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFileInput tests reading from named files (R1.3).
func TestDiffFileInput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsort")
	if err != nil {
		t.Skipf("reference binary gsort not in PATH: %v", err)
	}

	tmpDir := t.TempDir()

	// Create test input files.
	file1 := filepath.Join(tmpDir, "input1.txt")
	file2 := filepath.Join(tmpDir, "input2.txt")
	if err := os.WriteFile(file1, []byte("cherry\napple\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("banana\ndate\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.3: Single file input.
		{
			Name: "single_file",
			Args: []string{file1},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: Multiple file inputs merged.
		{
			Name: "multi_file",
			Args: []string{file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffOutputFile tests -o flag for output to file (R1.6).
func TestDiffOutputFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsort")
	if err != nil {
		t.Skipf("reference binary gsort not in PATH: %v", err)
	}

	// Test -o writing to a separate output file.
	t.Run("output_to_file", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		inputFile := filepath.Join(tmpDir, "input.txt")
		goOut := filepath.Join(tmpDir, "go_out.txt")
		refOut := filepath.Join(tmpDir, "ref_out.txt")

		inputData := []byte("cherry\napple\nbanana\n")
		if err := os.WriteFile(inputFile, inputData, 0o644); err != nil {
			t.Fatalf("writing test file: %v", err)
		}

		// Run Go binary.
		goCmd := exec.Command(goBin, "-o", goOut, inputFile)
		goCmd.Env = append(os.Environ(), "LC_ALL=C")
		if out, err := goCmd.CombinedOutput(); err != nil {
			t.Fatalf("go binary failed: %v\n%s", err, out)
		}

		// Run reference binary.
		refCmd := exec.Command(refBin, "-o", refOut, inputFile)
		refCmd.Env = append(os.Environ(), "LC_ALL=C")
		if out, err := refCmd.CombinedOutput(); err != nil {
			t.Fatalf("ref binary failed: %v\n%s", err, out)
		}

		goData, err := os.ReadFile(goOut)
		if err != nil {
			t.Fatalf("reading go output: %v", err)
		}
		refData, err := os.ReadFile(refOut)
		if err != nil {
			t.Fatalf("reading ref output: %v", err)
		}

		if string(goData) != string(refData) {
			t.Errorf("-o output mismatch:\nref: %q\ngot: %q", refData, goData)
		}
	})

	// Test -o with in-place sorting (output file same as input file).
	t.Run("inplace_sort", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		goFile := filepath.Join(tmpDir, "go_inplace.txt")
		refFile := filepath.Join(tmpDir, "ref_inplace.txt")

		inputData := []byte("cherry\napple\nbanana\n")
		if err := os.WriteFile(goFile, inputData, 0o644); err != nil {
			t.Fatalf("writing test file: %v", err)
		}
		if err := os.WriteFile(refFile, inputData, 0o644); err != nil {
			t.Fatalf("writing test file: %v", err)
		}

		// Run Go binary with in-place sort.
		goCmd := exec.Command(goBin, "-o", goFile, goFile)
		goCmd.Env = append(os.Environ(), "LC_ALL=C")
		if out, err := goCmd.CombinedOutput(); err != nil {
			t.Fatalf("go binary failed: %v\n%s", err, out)
		}

		// Run reference binary with in-place sort.
		refCmd := exec.Command(refBin, "-o", refFile, refFile)
		refCmd.Env = append(os.Environ(), "LC_ALL=C")
		if out, err := refCmd.CombinedOutput(); err != nil {
			t.Fatalf("ref binary failed: %v\n%s", err, out)
		}

		goData, err := os.ReadFile(goFile)
		if err != nil {
			t.Fatalf("reading go output: %v", err)
		}
		refData, err := os.ReadFile(refFile)
		if err != nil {
			t.Fatalf("reading ref output: %v", err)
		}

		if string(goData) != string(refData) {
			t.Errorf("-o inplace mismatch:\nref: %q\ngot: %q", refData, goData)
		}
	})
}
