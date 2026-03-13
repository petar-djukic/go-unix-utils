// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd028-uniq R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R3.4 (differential tests)
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinary is the Homebrew GNU uniq binary name.
const refBinary = "guniq"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinary)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinary, err)
	}

	tests := []testutils.DiffTest{
		// R1.1, R1.2: Basic adjacent dedup.
		{
			Name:  "basic_adjacent_dedup",
			Stdin: []byte("a\na\nb\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "all_unique",
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "all_same",
			Stdin: []byte("a\na\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "empty_input",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "single_line",
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "multiple_runs",
			Stdin: []byte("a\na\nb\nb\nc\nc\na\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: -c flag (count).
		{
			Name:  "count_flag",
			Args:  []string{"-c"},
			Stdin: []byte("a\na\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "count_flag_all_same",
			Args:  []string{"-c"},
			Stdin: []byte("x\nx\nx\nx\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "count_flag_all_unique",
			Args:  []string{"-c"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "count_long_flag",
			Args:  []string{"--count"},
			Stdin: []byte("a\na\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: -d flag (repeated only).
		{
			Name:  "repeated_flag",
			Args:  []string{"-d"},
			Stdin: []byte("a\na\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "repeated_flag_none",
			Args:  []string{"-d"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "repeated_long_flag",
			Args:  []string{"--repeated"},
			Stdin: []byte("a\na\nb\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: -u flag (unique only).
		{
			Name:  "unique_flag",
			Args:  []string{"-u"},
			Stdin: []byte("a\na\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "unique_flag_all_dupes",
			Args:  []string{"-u"},
			Stdin: []byte("a\na\nb\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "unique_long_flag",
			Args:  []string{"--unique"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Combined flags.
		{
			Name:  "count_and_repeated",
			Args:  []string{"-cd"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "count_and_unique",
			Args:  []string{"-cu"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: Case-sensitive comparison.
		{
			Name:  "case_sensitive",
			Stdin: []byte("A\na\nA\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Empty lines.
		{
			Name:  "empty_lines",
			Stdin: []byte("\n\n\na\n\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Lines with spaces.
		{
			Name:  "lines_with_spaces",
			Stdin: []byte("  a\n  a\n a\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -d prints only duplicate lines (one per run).
		{
			Name:  "r2_1_repeated_mixed",
			Args:  []string{"-d"},
			Stdin: []byte("a\na\na\nb\nc\nc\nd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_1_repeated_single_run",
			Args:  []string{"-d"},
			Stdin: []byte("x\nx\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: -D prints all lines of duplicate runs.
		{
			Name:  "r2_2_all_repeated_basic",
			Args:  []string{"-D"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_2_all_repeated_no_dupes",
			Args:  []string{"-D"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_2_all_repeated_all_same",
			Args:  []string{"-D"},
			Stdin: []byte("x\nx\nx\nx\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_2_all_repeated_long_flag",
			Args:  []string{"--all-repeated"},
			Stdin: []byte("a\na\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_2_all_repeated_empty",
			Args:  []string{"-D"},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_2_all_repeated_multiple_runs",
			Args:  []string{"-D"},
			Stdin: []byte("a\na\nb\nb\nc\nd\nd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: -u prints only unique lines.
		{
			Name:  "r2_3_unique_mixed",
			Args:  []string{"-u"},
			Stdin: []byte("a\na\nb\nc\nc\nd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_3_unique_all_unique",
			Args:  []string{"-u"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: -c count prefix.
		{
			Name:  "r2_4_count_mixed_runs",
			Args:  []string{"-c"},
			Stdin: []byte("a\na\na\nb\nc\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_4_count_single_line",
			Args:  []string{"-c"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: Flag combinations.
		{
			Name:  "r2_4_combo_count_repeated",
			Args:  []string{"-c", "-d"},
			Stdin: []byte("a\na\nb\nc\nc\nc\nd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_4_combo_count_unique",
			Args:  []string{"-c", "-u"},
			Stdin: []byte("a\na\nb\nc\nc\nc\nd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_4_combo_repeated_unique",
			Args:  []string{"-d", "-u"},
			Stdin: []byte("a\na\nb\nc\nc\nd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2_4_combo_all_three",
			Args:  []string{"-c", "-d", "-u"},
			Stdin: []byte("a\na\nb\nc\nc\nd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: -i case-insensitive comparison.
		{
			Name:  "r3_1_ignore_case_basic",
			Args:  []string{"-i"},
			Stdin: []byte("A\na\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_1_ignore_case_mixed",
			Args:  []string{"-i"},
			Stdin: []byte("Hello\nhello\nHELLO\nworld\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_1_ignore_case_no_match",
			Args:  []string{"-i"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_1_ignore_case_long_flag",
			Args:  []string{"--ignore-case"},
			Stdin: []byte("Foo\nfoo\nFOO\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_1_ignore_case_with_count",
			Args:  []string{"-i", "-c"},
			Stdin: []byte("A\na\nB\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_1_ignore_case_with_repeated",
			Args:  []string{"-i", "-d"},
			Stdin: []byte("A\na\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_1_ignore_case_with_unique",
			Args:  []string{"-i", "-u"},
			Stdin: []byte("A\na\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: -f N skip fields.
		{
			Name:  "r3_2_skip_fields_basic",
			Args:  []string{"-f", "1"},
			Stdin: []byte("a x\nb x\nc y\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_2_skip_fields_two",
			Args:  []string{"-f", "2"},
			Stdin: []byte("a b c\nd e c\nf g h\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_2_skip_fields_more_than_exist",
			Args:  []string{"-f", "5"},
			Stdin: []byte("a b\nc d\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_2_skip_fields_with_tabs",
			Args:  []string{"-f", "1"},
			Stdin: []byte("a\tx\nb\tx\nc\ty\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_2_skip_fields_leading_spaces",
			Args:  []string{"-f", "1"},
			Stdin: []byte("  a x\n  b x\n  c y\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_2_skip_fields_with_count",
			Args:  []string{"-f", "1", "-c"},
			Stdin: []byte("a x\nb x\nc y\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.3: -s N skip characters.
		{
			Name:  "r3_3_skip_chars_basic",
			Args:  []string{"-s", "1"},
			Stdin: []byte("ax\nbx\ncy\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_3_skip_chars_two",
			Args:  []string{"-s", "2"},
			Stdin: []byte("aax\nbbx\nccy\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_3_skip_chars_more_than_length",
			Args:  []string{"-s", "10"},
			Stdin: []byte("ab\ncd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_3_skip_chars_with_count",
			Args:  []string{"-s", "1", "-c"},
			Stdin: []byte("ax\nbx\ncy\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.4: -w N check only first N characters.
		{
			Name:  "r3_4_check_chars_basic",
			Args:  []string{"-w", "1"},
			Stdin: []byte("ax\nay\nbz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_4_check_chars_two",
			Args:  []string{"-w", "2"},
			Stdin: []byte("abc\nabd\nxyz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_4_check_chars_more_than_length",
			Args:  []string{"-w", "100"},
			Stdin: []byte("ab\nab\ncd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_4_check_chars_with_count",
			Args:  []string{"-w", "1", "-c"},
			Stdin: []byte("ax\nay\nbz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Combined R3 flags.
		{
			Name:  "r3_combined_skip_fields_and_chars",
			Args:  []string{"-f", "1", "-s", "1"},
			Stdin: []byte("a xhello\nb yhello\nc zworld\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_combined_skip_fields_and_check_chars",
			Args:  []string{"-f", "1", "-w", "1"},
			Stdin: []byte("a hello\nb howdy\nc xyz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_combined_skip_chars_and_check_chars",
			Args:  []string{"-s", "2", "-w", "1"},
			Stdin: []byte("xxabc\nyyaxy\nzzb00\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_combined_ignore_case_and_skip_fields",
			Args:  []string{"-i", "-f", "1"},
			Stdin: []byte("a Hello\nb hello\nc World\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_combined_all_options",
			Args:  []string{"-i", "-f", "1", "-s", "1", "-w", "2"},
			Stdin: []byte("a xHEL\nb yHEL\nc zWOR\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_ignore_case_all_repeated",
			Args:  []string{"-i", "-D"},
			Stdin: []byte("A\na\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_skip_fields_zero",
			Args:  []string{"-f", "0"},
			Stdin: []byte("a\na\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3_check_chars_zero",
			Args:  []string{"-w", "0"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
