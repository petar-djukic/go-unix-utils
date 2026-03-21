// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd055-tail R1.1–R1.4, R2.1–R2.3 differential tests:
// line-count mode, byte-count mode, stdin reading, -n/-c flags,
// +NUM offset, and suffix multipliers.
package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtail")
	if err != nil {
		t.Skipf("reference binary gtail not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R1.1: default last 10 lines
			Name:  "default_last_10_lines",
			Stdin: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n13\n14\n15\n"),
		},
		{
			// R1.1: fewer than 10 lines prints all
			Name:  "fewer_than_10_lines",
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R1.2: -n 5 prints last 5 lines
			Name:  "n_flag_last_5",
			Args:  []string{"-n", "5"},
			Stdin: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"),
		},
		{
			// R1.2: --lines=3
			Name:  "lines_equals_3",
			Args:  []string{"--lines=3"},
			Stdin: []byte("a\nb\nc\nd\ne\nf\n"),
		},
		{
			// R1.2: -n1 attached form
			Name:  "n_attached_1",
			Args:  []string{"-n1"},
			Stdin: []byte("first\nsecond\nthird\n"),
		},
		{
			// R1.3: -n +3 prints from line 3 onward
			Name:  "n_plus_from_line_3",
			Args:  []string{"-n", "+3"},
			Stdin: []byte("1\n2\n3\n4\n5\n"),
		},
		{
			// R1.3: -n +1 prints all lines
			Name:  "n_plus_1_all_lines",
			Args:  []string{"-n", "+1"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R1.3: -n +100 on short input prints nothing
			Name:  "n_plus_beyond_end",
			Args:  []string{"-n", "+100"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R1.4: empty stdin
			Name:  "empty_stdin",
			Stdin: []byte{},
		},
		{
			// R1.4: stdin with - arg
			Name:  "dash_as_stdin",
			Args:  []string{"-"},
			Stdin: []byte("line1\nline2\nline3\n"),
		},
		{
			// R1.1: input without trailing newline
			Name:  "no_trailing_newline",
			Stdin: []byte("a\nb\nc"),
		},
		{
			// R1.2: -n 0 prints nothing
			Name:  "n_zero",
			Args:  []string{"-n", "0"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			// R1.1: exactly 10 lines
			Name:  "exactly_10_lines",
			Stdin: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"),
		},
		// R2.1: byte-count mode (-c NUM)
		{
			Name:  "c_last_5_bytes",
			Args:  []string{"-c", "5"},
			Stdin: []byte("abcdefghij"),
		},
		{
			Name:  "c_last_3_bytes",
			Args:  []string{"-c", "3"},
			Stdin: []byte("hello world\n"),
		},
		{
			// R2.1: -c exceeds input length
			Name:  "c_exceeds_input",
			Args:  []string{"-c", "100"},
			Stdin: []byte("short"),
		},
		{
			// R2.1: -c 0 prints nothing
			Name:  "c_zero",
			Args:  []string{"-c", "0"},
			Stdin: []byte("abc"),
		},
		{
			// R2.1: -c attached form
			Name:  "c_attached_10",
			Args:  []string{"-c10"},
			Stdin: []byte("abcdefghijklmnop"),
		},
		{
			// R2.1: --bytes= form
			Name:  "bytes_equals_4",
			Args:  []string{"--bytes=4"},
			Stdin: []byte("abcdefgh"),
		},
		// R2.2: byte-count from top (+NUM)
		{
			Name:  "c_plus_5_from_byte",
			Args:  []string{"-c", "+5"},
			Stdin: []byte("abcdefghij"),
		},
		{
			// R2.2: -c +1 prints all bytes
			Name:  "c_plus_1_all",
			Args:  []string{"-c", "+1"},
			Stdin: []byte("hello"),
		},
		{
			// R2.2: -c +N beyond end prints nothing
			Name:  "c_plus_beyond_end",
			Args:  []string{"-c", "+100"},
			Stdin: []byte("short"),
		},
		// R2.3: suffix multipliers
		{
			// R2.3: b suffix (512 bytes)
			Name:  "c_suffix_b",
			Args:  []string{"-c", "1b"},
			Stdin: []byte(strings.Repeat("x", 1024)),
		},
		{
			// R2.3: K suffix (1024 bytes) - request 2K from 3072 bytes
			Name:  "c_suffix_K",
			Args:  []string{"-c", "2K"},
			Stdin: []byte(strings.Repeat("y", 3072)),
		},
		{
			// R2.3: kB suffix (1000 bytes)
			Name:  "c_suffix_kB",
			Args:  []string{"-c", "1kB"},
			Stdin: []byte(strings.Repeat("z", 2000)),
		},
		// R2.1: -c and -n mutual exclusivity (last wins)
		{
			Name:  "n_then_c_last_wins",
			Args:  []string{"-n", "2", "-c", "5"},
			Stdin: []byte("abcdefghij\nklmnopqrst\n"),
		},
		{
			Name:  "c_then_n_last_wins",
			Args:  []string{"-c", "5", "-n", "1"},
			Stdin: []byte("abcdefghij\nklmnopqrst\n"),
		},
		// R2.1: byte mode with empty input
		{
			Name:  "c_empty_input",
			Args:  []string{"-c", "10"},
			Stdin: []byte{},
		},
		// R2.2: +NUM with newlines in byte mode
		{
			Name:  "c_plus_with_newlines",
			Args:  []string{"-c", "+4"},
			Stdin: []byte("ab\ncd\nef\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
