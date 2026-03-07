// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cut against the GNU reference binary (gcut).
//
// Implements prd026-cut acceptance criteria AC1-AC7 via testutils.RunDiffTests.
package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcut")
	if err != nil {
		t.Skipf("reference binary gcut not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: -b byte range N-M.
		{
			Name:  "cut_byte_range",
			Args:  []string{"-b", "2-4"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.1: -b open range N-.
		{
			Name:  "cut_byte_open_range",
			Args:  []string{"-b", "3-"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.1: -b closed range -M.
		{
			Name:  "cut_byte_start_range",
			Args:  []string{"-b", "-3"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.1: -b single position.
		{
			Name:  "cut_byte_single",
			Args:  []string{"-b", "2"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.1: -b comma-separated ranges.
		{
			Name:  "cut_byte_comma_ranges",
			Args:  []string{"-b", "1,3,5"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.2: -c character selection (equivalent to -b under LC_ALL=C).
		{
			Name:  "cut_char_range",
			Args:  []string{"-c", "2-4"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.4: Line shorter than selected range.
		{
			Name:  "cut_byte_short_line",
			Args:  []string{"-b", "3-10"},
			Stdin: []byte("ab\n"),
		},
		// R2.1, R2.2: -d and -f field selection.
		{
			Name:  "cut_field_with_delimiter",
			Args:  []string{"-d:", "-f2"},
			Stdin: []byte("a:b:c\n"),
		},
		// R2.1: -f multiple fields.
		{
			Name:  "cut_field_multiple",
			Args:  []string{"-d:", "-f1,3"},
			Stdin: []byte("a:b:c\n"),
		},
		// R2.1: -f field range.
		{
			Name:  "cut_field_range",
			Args:  []string{"-d:", "-f2-3"},
			Stdin: []byte("a:b:c:d\n"),
		},
		// R2.1: -f open-ended field range.
		{
			Name:  "cut_field_open_range",
			Args:  []string{"-d:", "-f2-"},
			Stdin: []byte("a:b:c:d\n"),
		},
		// R2.3: -s suppresses lines without delimiter.
		{
			Name:  "cut_suppress_no_delimiter",
			Args:  []string{"-d:", "-f2", "-s"},
			Stdin: []byte("no-delimiter\na:b\n"),
		},
		// R2.3: Without -s, lines without delimiter are passed through.
		{
			Name:  "cut_no_delimiter_passthrough",
			Args:  []string{"-d:", "-f2"},
			Stdin: []byte("no-delimiter\n"),
		},
		// R2.4: --output-delimiter replaces separator in output.
		{
			Name:  "cut_output_delimiter",
			Args:  []string{"-d:", "-f1,3", "--output-delimiter=|"},
			Stdin: []byte("a:b:c\n"),
		},
		// R2.4: --output-delimiter with multi-char string.
		{
			Name:  "cut_output_delimiter_multi",
			Args:  []string{"-d:", "-f1,2,3", "--output-delimiter=, "},
			Stdin: []byte("a:b:c\n"),
		},
		// R3.1: --complement with -f inverts field selection.
		{
			Name:  "cut_complement_fields",
			Args:  []string{"-d:", "--complement", "-f2"},
			Stdin: []byte("a:b:c\n"),
		},
		// R3.1: --complement with -b inverts byte selection.
		{
			Name:  "cut_complement_bytes",
			Args:  []string{"--complement", "-b", "2-4"},
			Stdin: []byte("abcdef\n"),
		},
		// Default tab delimiter with -f.
		{
			Name:  "cut_default_tab_delimiter",
			Args:  []string{"-f2"},
			Stdin: []byte("a\tb\tc\n"),
		},
		// Multiple lines.
		{
			Name:  "cut_multiple_lines",
			Args:  []string{"-d:", "-f1"},
			Stdin: []byte("a:b\nc:d\ne:f\n"),
		},
		// Empty input.
		{
			Name:  "cut_empty_input",
			Args:  []string{"-b", "1"},
			Stdin: []byte{},
		},
		// -f with --complement and --output-delimiter.
		{
			Name:  "cut_complement_output_delim",
			Args:  []string{"-d:", "-f2", "--complement", "--output-delimiter=|"},
			Stdin: []byte("a:b:c:d\n"),
		},
		// -b with overlapping ranges.
		{
			Name:  "cut_byte_overlapping_ranges",
			Args:  []string{"-b", "1-3,2-5"},
			Stdin: []byte("abcdef\n"),
		},
		// -f single field from line with many fields.
		{
			Name:  "cut_field_last",
			Args:  []string{"-d:", "-f4"},
			Stdin: []byte("a:b:c:d:e\n"),
		},
		// -f requesting field beyond available.
		{
			Name:  "cut_field_beyond_available",
			Args:  []string{"-d:", "-f10"},
			Stdin: []byte("a:b:c\n"),
		},
		// Stdin read (no files, pipe mode).
		{
			Name:  "cut_stdin_pipe",
			Args:  []string{"-d,", "-f2"},
			Stdin: []byte("x,y,z\n"),
		},
		// Error: missing file should exit 1.
		{
			Name:      "cut_missing_file",
			Args:      []string{"-b", "1", "/nonexistent/file"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrBinaryNameNormalizer},
		},
		// -s only with -f, no delimiter line is suppressed.
		{
			Name:  "cut_suppress_all_lines",
			Args:  []string{"-d:", "-f1", "-s"},
			Stdin: []byte("nodlim\n"),
		},
		// -b -M start range.
		{
			Name:  "cut_byte_dash_m",
			Args:  []string{"-b", "-4"},
			Stdin: []byte("abcdefgh\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// stderrBinaryNameNormalizer replaces the binary name prefix in stderr so
// messages from both binaries compare equal.
var stderrBinaryNameNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gcut:"), []byte("cut:"))
	return b
}
