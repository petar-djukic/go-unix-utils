// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd028-uniq R1.1–R1.4: default adjacent-duplicate
// suppression, stdin/file input, case-sensitive comparison.
// Differential tests for prd028-uniq R2.1–R2.4: -d, -D, -u, -c output selection.
// Differential tests for prd028-uniq R3.1–R3.4: -i, -f, -s, -w comparison options.
package main

import (
	"os/exec"
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
		// R1.1: suppress adjacent duplicates, keep first of each run
		{
			Name:  "adjacent_duplicates",
			Stdin: []byte("a\na\nb\na\n"),
		},
		// R1.2: single-occurrence lines pass through
		{
			Name:  "all_unique",
			Stdin: []byte("a\nb\nc\n"),
		},
		// R1.2: non-adjacent duplicates are unaffected
		{
			Name:  "non_adjacent_duplicates",
			Stdin: []byte("a\nb\na\nb\n"),
		},
		// R1.1: multiple runs of duplicates
		{
			Name:  "multiple_runs",
			Stdin: []byte("x\nx\nx\ny\ny\nz\nz\nz\nz\n"),
		},
		// R1.3: empty input produces empty output
		{
			Name:  "empty_input",
			Stdin: []byte(""),
		},
		// R1.4: case-sensitive comparison
		{
			Name:  "case_sensitive",
			Stdin: []byte("A\na\nA\n"),
		},
		// R1.1: single line
		{
			Name:  "single_line",
			Stdin: []byte("hello\n"),
		},
		// R1.1: all identical lines
		{
			Name:  "all_identical",
			Stdin: []byte("foo\nfoo\nfoo\n"),
		},
		// R1.4: lines with trailing spaces are distinct
		{
			Name:  "trailing_spaces_differ",
			Stdin: []byte("abc\nabc \nabc\n"),
		},
		// R1.1: blank lines are adjacent duplicates
		{
			Name:  "blank_lines",
			Stdin: []byte("\n\n\na\n\n\n"),
		},
		// R1.3: "-" means stdin
		{
			Name:  "dash_means_stdin",
			Args:  []string{"-"},
			Stdin: []byte("a\na\nb\n"),
		},

		// R2.1: -d prints only duplicate runs (one copy)
		{
			Name:  "d_flag_basic",
			Args:  []string{"-d"},
			Stdin: []byte("a\na\nb\n"),
		},
		// R2.1: -d with no duplicates produces no output
		{
			Name:  "d_flag_no_dups",
			Args:  []string{"-d"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R2.1: -d multiple duplicate runs
		{
			Name:  "d_flag_multiple_runs",
			Args:  []string{"-d"},
			Stdin: []byte("a\na\nb\nc\nc\nd\n"),
		},
		// R2.1: -d all identical
		{
			Name:  "d_flag_all_identical",
			Args:  []string{"-d"},
			Stdin: []byte("x\nx\nx\n"),
		},

		// R2.2: -D prints all lines of duplicate runs
		{
			Name:  "D_flag_basic",
			Args:  []string{"-D"},
			Stdin: []byte("a\na\nb\n"),
		},
		// R2.2: -D with no duplicates produces no output
		{
			Name:  "D_flag_no_dups",
			Args:  []string{"-D"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R2.2: -D multiple duplicate runs
		{
			Name:  "D_flag_multiple_runs",
			Args:  []string{"-D"},
			Stdin: []byte("a\na\nb\nc\nc\nc\nd\n"),
		},
		// R2.2: -D all identical
		{
			Name:  "D_flag_all_identical",
			Args:  []string{"-D"},
			Stdin: []byte("x\nx\nx\n"),
		},

		// R2.3: -u prints only unique lines (runs of one)
		{
			Name:  "u_flag_basic",
			Args:  []string{"-u"},
			Stdin: []byte("a\na\nb\n"),
		},
		// R2.3: -u all unique
		{
			Name:  "u_flag_all_unique",
			Args:  []string{"-u"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R2.3: -u all duplicates produces no output
		{
			Name:  "u_flag_no_unique",
			Args:  []string{"-u"},
			Stdin: []byte("a\na\nb\nb\n"),
		},
		// R2.3: -u mixed
		{
			Name:  "u_flag_mixed",
			Args:  []string{"-u"},
			Stdin: []byte("a\nb\nb\nc\nd\nd\ne\n"),
		},

		// R2.4: -c prefixes each line with count
		{
			Name:  "c_flag_basic",
			Args:  []string{"-c"},
			Stdin: []byte("a\na\nb\n"),
		},
		// R2.4: -c single lines
		{
			Name:  "c_flag_all_unique",
			Args:  []string{"-c"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R2.4: -c large run
		{
			Name:  "c_flag_large_run",
			Args:  []string{"-c"},
			Stdin: []byte("x\nx\nx\nx\nx\ny\n"),
		},
		// R2.4: -c empty input
		{
			Name:  "c_flag_empty",
			Args:  []string{"-c"},
			Stdin: []byte(""),
		},

		// R2.1 + R2.4: -cd combined
		{
			Name:  "cd_combined",
			Args:  []string{"-cd"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R2.3 + R2.4: -cu combined
		{
			Name:  "cu_combined",
			Args:  []string{"-cu"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R2.1 + R2.3: -du (both filters cancel; matches GNU behavior)
		{
			Name:  "du_combined",
			Args:  []string{"-du"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},

		// R3.1: -i case-insensitive comparison
		{
			Name:  "i_flag_basic",
			Args:  []string{"-i"},
			Stdin: []byte("A\na\nb\n"),
		},
		// R3.1: -i all same when case-folded
		{
			Name:  "i_flag_all_same",
			Args:  []string{"-i"},
			Stdin: []byte("ABC\nabc\nAbC\n"),
		},
		// R3.1: -i no effect when already equal
		{
			Name:  "i_flag_no_effect",
			Args:  []string{"-i"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R3.1: -i with -c
		{
			Name:  "ic_combined",
			Args:  []string{"-ic"},
			Stdin: []byte("A\na\nB\nb\nb\n"),
		},
		// R3.1: -i with -d
		{
			Name:  "id_combined",
			Args:  []string{"-id"},
			Stdin: []byte("A\na\nb\nc\nC\n"),
		},

		// R3.2: -f N skip first N fields before comparing
		{
			Name:  "f_flag_skip_one",
			Args:  []string{"-f", "1"},
			Stdin: []byte("1 a\n2 a\n3 b\n"),
		},
		// R3.2: -f inline value
		{
			Name:  "f_flag_inline",
			Args:  []string{"-f1"},
			Stdin: []byte("x hello\ny hello\nz world\n"),
		},
		// R3.2: -f skip two fields
		{
			Name:  "f_flag_skip_two",
			Args:  []string{"-f", "2"},
			Stdin: []byte("a b c\nx y c\np q d\n"),
		},
		// R3.2: -f 0 has no effect
		{
			Name:  "f_flag_zero",
			Args:  []string{"-f", "0"},
			Stdin: []byte("a\na\nb\n"),
		},
		// R3.2: -f with tab-separated fields
		{
			Name:  "f_flag_tabs",
			Args:  []string{"-f", "1"},
			Stdin: []byte("1\ta\n2\ta\n3\tb\n"),
		},
		// R3.2: -f with more fields to skip than exist
		{
			Name:  "f_flag_skip_beyond",
			Args:  []string{"-f", "5"},
			Stdin: []byte("a b\nc d\ne f\n"),
		},

		// R3.3: -s N skip first N characters before comparing
		{
			Name:  "s_flag_basic",
			Args:  []string{"-s", "2"},
			Stdin: []byte("xxhello\nyyhello\nzzworld\n"),
		},
		// R3.3: -s inline value
		{
			Name:  "s_flag_inline",
			Args:  []string{"-s1"},
			Stdin: []byte("xa\nya\nzb\n"),
		},
		// R3.3: -s 0 has no effect
		{
			Name:  "s_flag_zero",
			Args:  []string{"-s", "0"},
			Stdin: []byte("a\na\nb\n"),
		},
		// R3.3: -s skip beyond line length
		{
			Name:  "s_flag_skip_beyond",
			Args:  []string{"-s", "100"},
			Stdin: []byte("abc\ndef\nghi\n"),
		},

		// R3.4: -w N compare only first N characters
		{
			Name:  "w_flag_basic",
			Args:  []string{"-w", "3"},
			Stdin: []byte("abcXXX\nabcYYY\ndefZZZ\n"),
		},
		// R3.4: -w inline value
		{
			Name:  "w_flag_inline",
			Args:  []string{"-w1"},
			Stdin: []byte("apple\naardvark\nbanana\n"),
		},
		// R3.4: -w 0 means compare zero chars (all lines equal)
		{
			Name:  "w_flag_zero",
			Args:  []string{"-w", "0"},
			Stdin: []byte("abc\ndef\nghi\n"),
		},
		// R3.4: -w larger than line length
		{
			Name:  "w_flag_large",
			Args:  []string{"-w", "100"},
			Stdin: []byte("abc\nabc\ndef\n"),
		},

		// R3.2 + R3.1: -f and -i combined
		{
			Name:  "fi_combined",
			Args:  []string{"-f", "1", "-i"},
			Stdin: []byte("1 ABC\n2 abc\n3 DEF\n"),
		},
		// R3.2 + R3.3: -f and -s combined
		{
			Name:  "fs_combined",
			Args:  []string{"-f", "1", "-s", "1"},
			Stdin: []byte("a xhello\nb xhello\nc yworld\n"),
		},
		// R3.2 + R3.4: -f and -w combined
		{
			Name:  "fw_combined",
			Args:  []string{"-f", "1", "-w", "2"},
			Stdin: []byte("x abXXX\ny abYYY\nz cdZZZ\n"),
		},
		// R3.3 + R3.4: -s and -w combined
		{
			Name:  "sw_combined",
			Args:  []string{"-s", "2", "-w", "3"},
			Stdin: []byte("xxabcQQ\nyyabcRR\nzzdefSS\n"),
		},
		// R3.1 + R3.4: -i and -w combined
		{
			Name:  "iw_combined",
			Args:  []string{"-i", "-w", "3"},
			Stdin: []byte("ABCxxx\nabcyyy\nDEFzzz\n"),
		},
		// R3.1 + R3.2 + R3.4: -i -f -w combined
		{
			Name:  "ifw_combined",
			Args:  []string{"-i", "-f", "1", "-w", "2"},
			Stdin: []byte("1 ABxx\n2 abxx\n3 CDyy\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
