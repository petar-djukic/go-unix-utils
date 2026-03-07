// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cut against the GNU reference binary (gcut).
// Implements prd026-cut R1-R4 verification.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrEraser clears stderr so error message wording and program name
// differences between our binary and gcut do not cause false failures.
// Exit code comparison still validates the error behavior.
var stderrEraser testutils.NormalizeFunc = func(b []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcut")
	if err != nil {
		t.Skipf("reference binary gcut not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: -b byte range N-M
		{
			Name:  "byte_range",
			Args:  []string{"-b", "2-4"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.1: -b open range N-
		{
			Name:  "byte_open_range",
			Args:  []string{"-b", "3-"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.1: -b range -M (from start)
		{
			Name:  "byte_range_from_start",
			Args:  []string{"-b", "-3"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.1: -b single position
		{
			Name:  "byte_single",
			Args:  []string{"-b", "2"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.1: -b comma-separated list
		{
			Name:  "byte_comma_list",
			Args:  []string{"-b", "1,3,5"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.2: -c character range (equivalent to -b under LC_ALL=C)
		{
			Name:  "char_range",
			Args:  []string{"-c", "2-4"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.4: line shorter than selected range
		{
			Name:  "short_line",
			Args:  []string{"-b", "3-10"},
			Stdin: []byte("ab\n"),
		},
		// R2.1: -f field selection with -d delimiter
		{
			Name:  "field_with_delimiter",
			Args:  []string{"-d:", "-f2"},
			Stdin: []byte("a:b:c\n"),
		},
		// R2.1: -f multiple fields
		{
			Name:  "field_multiple",
			Args:  []string{"-d:", "-f1,3"},
			Stdin: []byte("a:b:c\n"),
		},
		// R2.1: -f field range
		{
			Name:  "field_range",
			Args:  []string{"-d:", "-f2-3"},
			Stdin: []byte("a:b:c:d\n"),
		},
		// R2.2: default tab delimiter
		{
			Name:  "field_tab_delimiter",
			Args:  []string{"-f2"},
			Stdin: []byte("a\tb\tc\n"),
		},
		// R2.3: -s suppress lines without delimiter
		{
			Name:  "suppress_no_delimiter",
			Args:  []string{"-d:", "-f2", "-s"},
			Stdin: []byte("no-delimiter\na:b\n"),
		},
		// R2.3: without -s, lines without delimiter pass through
		{
			Name:  "no_delimiter_passthrough",
			Args:  []string{"-d:", "-f2"},
			Stdin: []byte("no-delimiter\n"),
		},
		// R2.4: --output-delimiter
		{
			Name:  "output_delimiter",
			Args:  []string{"-d:", "-f1,3", "--output-delimiter=|"},
			Stdin: []byte("a:b:c\n"),
		},
		// R3.1: --complement with -f
		{
			Name:  "complement_fields",
			Args:  []string{"-d:", "--complement", "-f2"},
			Stdin: []byte("a:b:c\n"),
		},
		// R3.1: --complement with -b
		{
			Name:  "complement_bytes",
			Args:  []string{"--complement", "-b", "2-4"},
			Stdin: []byte("abcdef\n"),
		},
		// Multiple lines
		{
			Name:  "multiple_lines",
			Args:  []string{"-d:", "-f1"},
			Stdin: []byte("a:b:c\nx:y:z\n"),
		},
		// -z zero-terminated lines
		{
			Name:  "zero_terminated",
			Args:  []string{"-z", "-d:", "-f2"},
			Stdin: []byte("a:b:c\x00x:y:z\x00"),
		},
		// Error: no list specified
		{
			Name:      "error_no_list",
			Args:      []string{},
			Stdin:     []byte("hello\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrEraser},
		},
		// Error: conflicting -b and -f
		{
			Name:      "error_conflicting_flags",
			Args:      []string{"-b1", "-f1"},
			Stdin:     []byte("hello\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrEraser},
		},
		// -f with open range N-
		{
			Name:  "field_open_range",
			Args:  []string{"-d:", "-f2-"},
			Stdin: []byte("a:b:c:d\n"),
		},
		// --output-delimiter with -b
		{
			Name:  "output_delimiter_bytes",
			Args:  []string{"-b", "1,3,5", "--output-delimiter=_"},
			Stdin: []byte("abcdef\n"),
		},
		// --complement with --output-delimiter
		{
			Name:  "complement_output_delimiter",
			Args:  []string{"-d:", "-f2", "--complement", "--output-delimiter=|"},
			Stdin: []byte("a:b:c\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
