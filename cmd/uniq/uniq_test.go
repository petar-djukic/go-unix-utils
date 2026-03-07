// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/uniq against the GNU reference binary (guniq).
//
// Implements prd028-uniq acceptance criteria AC1-AC7 via testutils.RunDiffTests.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: Default adjacent dedup.
		{
			Name:  "uniq_default_dedup",
			Args:  []string{},
			Stdin: []byte("a\na\nb\na\n"),
		},
		// R1.1: Single line input.
		{
			Name:  "uniq_single_line",
			Args:  []string{},
			Stdin: []byte("only\n"),
		},
		// R1: Empty input.
		{
			Name:  "uniq_empty_input",
			Args:  []string{},
			Stdin: []byte{},
		},
		// R2.4: -c count prefix.
		{
			Name:  "uniq_count",
			Args:  []string{"-c"},
			Stdin: []byte("a\na\nb\n"),
		},
		// R2.1: -d duplicates only.
		{
			Name:  "uniq_duplicates_only",
			Args:  []string{"-d"},
			Stdin: []byte("a\na\nb\n"),
		},
		// R2.2: -D all duplicates (default method=none).
		{
			Name:  "uniq_all_duplicates",
			Args:  []string{"-D"},
			Stdin: []byte("a\na\nb\n"),
		},
		// R2.2: --all-repeated=prepend.
		{
			Name:  "uniq_all_repeated_prepend",
			Args:  []string{"--all-repeated=prepend"},
			Stdin: []byte("a\na\nb\nb\nc\n"),
		},
		// R2.2: --all-repeated=separate.
		{
			Name:  "uniq_all_repeated_separate",
			Args:  []string{"--all-repeated=separate"},
			Stdin: []byte("a\na\nb\nb\nc\n"),
		},
		// R2.3: -u unique only.
		{
			Name:  "uniq_unique_only",
			Args:  []string{"-u"},
			Stdin: []byte("a\na\nb\n"),
		},
		// R3.1: -i case-insensitive.
		{
			Name:  "uniq_case_insensitive",
			Args:  []string{"-i"},
			Stdin: []byte("A\na\nb\n"),
		},
		// R3.2: -f 1 skip first field.
		{
			Name:  "uniq_skip_fields",
			Args:  []string{"-f", "1"},
			Stdin: []byte("key1 val\nkey2 val\nkey3 other\n"),
		},
		// R3.3: -s 2 skip first 2 characters.
		{
			Name:  "uniq_skip_chars",
			Args:  []string{"-s", "2"},
			Stdin: []byte("xxhello\nyyhello\nzzworld\n"),
		},
		// R3.4: -w 3 compare only first 3 characters.
		{
			Name:  "uniq_check_chars",
			Args:  []string{"-w", "3"},
			Stdin: []byte("abcX\nabcY\ndefZ\n"),
		},
		// -z zero-terminated mode.
		{
			Name:  "uniq_zero_terminated",
			Args:  []string{"-z"},
			Stdin: []byte("a\x00a\x00b\x00"),
		},
		// --group=separate (default).
		{
			Name:  "uniq_group_separate",
			Args:  []string{"--group=separate"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// --group=prepend.
		{
			Name:  "uniq_group_prepend",
			Args:  []string{"--group=prepend"},
			Stdin: []byte("a\na\nb\n"),
		},
		// --group=append.
		{
			Name:  "uniq_group_append",
			Args:  []string{"--group=append"},
			Stdin: []byte("a\na\nb\n"),
		},
		// --group=both.
		{
			Name:  "uniq_group_both",
			Args:  []string{"--group=both"},
			Stdin: []byte("a\na\nb\n"),
		},
		// Combination: -c -i (count + case-insensitive).
		{
			Name:  "uniq_count_ignore_case",
			Args:  []string{"-c", "-i"},
			Stdin: []byte("Hello\nhello\nWorld\n"),
		},
		// Combination: -d -f (duplicates + field skip).
		{
			Name:  "uniq_dup_skip_fields",
			Args:  []string{"-d", "-f", "1"},
			Stdin: []byte("x abc\ny abc\nz def\n"),
		},
		// Combination: -u -s -w (unique + skip chars + check chars).
		{
			Name:  "uniq_unique_skip_check",
			Args:  []string{"-u", "-s", "1", "-w", "2"},
			Stdin: []byte("xab1\nyab2\nzcd3\n"),
		},
		// All lines identical.
		{
			Name:  "uniq_all_identical",
			Args:  []string{},
			Stdin: []byte("a\na\na\n"),
		},
		// All lines unique.
		{
			Name:  "uniq_all_unique",
			Args:  []string{},
			Stdin: []byte("a\nb\nc\n"),
		},
		// -c with all identical lines.
		{
			Name:  "uniq_count_all_identical",
			Args:  []string{"-c"},
			Stdin: []byte("a\na\na\n"),
		},
		// -d with no duplicates.
		{
			Name:  "uniq_dup_no_matches",
			Args:  []string{"-d"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// -u with all duplicates.
		{
			Name:  "uniq_unique_no_matches",
			Args:  []string{"-u"},
			Stdin: []byte("a\na\nb\nb\n"),
		},
		// Combined short flags: -ci.
		{
			Name:  "uniq_combined_ci",
			Args:  []string{"-ci"},
			Stdin: []byte("A\na\nB\n"),
		},
		// -f with tab-separated fields.
		{
			Name:  "uniq_skip_fields_tab",
			Args:  []string{"-f", "1"},
			Stdin: []byte("k1\tval\nk2\tval\nk3\tother\n"),
		},
		// Multiple runs of duplicates with -c.
		{
			Name:  "uniq_count_multi_runs",
			Args:  []string{"-c"},
			Stdin: []byte("a\na\nb\nb\nb\nc\n"),
		},
		// -D with no duplicates produces no output.
		{
			Name:  "uniq_all_dup_no_matches",
			Args:  []string{"-D"},
			Stdin: []byte("a\nb\nc\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFileOperand verifies reading from an input file operand.
func TestDiffFileOperand(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(inputPath, []byte("a\na\nb\n"), 0o644); err != nil {
		t.Fatalf("writing input file: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:    "uniq_file_operand",
			Args:    []string{inputPath},
			WorkDir: dir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffOutputFile verifies writing to an output file operand.
func TestDiffOutputFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	// Run reference binary.
	refDir := t.TempDir()
	refInput := filepath.Join(refDir, "in.txt")
	refOutput := filepath.Join(refDir, "out.txt")
	if err := os.WriteFile(refInput, []byte("x\nx\ny\n"), 0o644); err != nil {
		t.Fatalf("writing ref input: %v", err)
	}

	refCmd := exec.Command(refBin, refInput, refOutput)
	refCmd.Env = append(os.Environ(), "LC_ALL=C")
	if out, err := refCmd.CombinedOutput(); err != nil {
		t.Fatalf("reference binary failed: %v\n%s", err, out)
	}
	refResult, err := os.ReadFile(refOutput)
	if err != nil {
		t.Fatalf("reading ref output: %v", err)
	}

	// Run Go binary.
	goDir := t.TempDir()
	goInput := filepath.Join(goDir, "in.txt")
	goOutput := filepath.Join(goDir, "out.txt")
	if err := os.WriteFile(goInput, []byte("x\nx\ny\n"), 0o644); err != nil {
		t.Fatalf("writing go input: %v", err)
	}

	goCmd := exec.Command(goBin, goInput, goOutput)
	goCmd.Env = append(os.Environ(), "LC_ALL=C")
	if out, err := goCmd.CombinedOutput(); err != nil {
		t.Fatalf("go binary failed: %v\n%s", err, out)
	}
	goResult, err := os.ReadFile(goOutput)
	if err != nil {
		t.Fatalf("reading go output: %v", err)
	}

	if !bytes.Equal(refResult, goResult) {
		t.Errorf("output file divergence\nref: %q\ngo:  %q", refResult, goResult)
	}
}
