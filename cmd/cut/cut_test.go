// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd026-cut R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R4.1, R4.2, R4.3, R4.4
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests comparing the Go cut binary against the
// GNU reference binary (gcut) via pkg/testutils.RunDiffTests.
//
// R4.1: Byte-for-byte comparison via RunDiffTests.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcut")
	if err != nil {
		t.Skipf("reference binary gcut not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// --- R1.1: Byte selection (-b) ---
		{
			Name:  "bytes_single",
			Args:  []string{"-b1"},
			Stdin: []byte("abcdef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// AC5: -b1-5 extracts first 5 bytes.
			Name:  "bytes_range",
			Args:  []string{"-b1-5"},
			Stdin: []byte("abcdefgh\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "bytes_open_end",
			Args:  []string{"-b3-"},
			Stdin: []byte("abcdef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "bytes_open_start",
			Args:  []string{"-b-3"},
			Stdin: []byte("abcdef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "bytes_multiple_ranges",
			Args:  []string{"-b1,3,5"},
			Stdin: []byte("abcdef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "bytes_mixed_ranges",
			Args:  []string{"-b1-2,4-5"},
			Stdin: []byte("abcdef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.4: Short line — out-of-range positions produce nothing.
			Name:  "bytes_short_line",
			Args:  []string{"-b5-10"},
			Stdin: []byte("abc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.3: Newlines are not counted; they terminate each output line.
			Name:  "bytes_multiline",
			Args:  []string{"-b1-3"},
			Stdin: []byte("abcdef\nxyz\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R1.2: Character selection (-c) under LC_ALL=C ---
		{
			Name:  "chars_range",
			Args:  []string{"-c2-4"},
			Stdin: []byte("abcdef\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R2.1: Field selection (-f) with default TAB delimiter ---
		{
			// AC3: -f1 with TAB-delimited input.
			Name:  "fields_tab_default",
			Args:  []string{"-f1"},
			Stdin: []byte("first\tsecond\tthird\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "fields_tab_multiple",
			Args:  []string{"-f1,3"},
			Stdin: []byte("a\tb\tc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "fields_tab_range",
			Args:  []string{"-f2-3"},
			Stdin: []byte("a\tb\tc\td\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "fields_tab_open_end",
			Args:  []string{"-f2-"},
			Stdin: []byte("a\tb\tc\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R2.2: Custom delimiter (-d) ---
		{
			// AC4: -d: -f1 extracts first colon-delimited field.
			Name:  "fields_colon_delim",
			Args:  []string{"-d:", "-f1"},
			Stdin: []byte("root:x:0:0\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "fields_colon_f1_f3",
			Args:  []string{"-d:", "-f1,3"},
			Stdin: []byte("a:b:c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "fields_pipe_delim",
			Args:  []string{"-d|", "-f2"},
			Stdin: []byte("foo|bar|baz\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R2.3: Suppress lines without delimiter (-s) ---
		{
			// AC6: -s -f1 suppresses lines that contain no delimiter.
			Name:  "suppress_no_delim",
			Args:  []string{"-d:", "-s", "-f1"},
			Stdin: []byte("no-delimiter\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "suppress_mixed_lines",
			Args:  []string{"-d:", "-s", "-f1"},
			Stdin: []byte("a:b\nno-delim\nc:d\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// Without -s, line without delimiter is printed unchanged.
			Name:  "no_suppress_no_delim",
			Args:  []string{"-d:", "-f1"},
			Stdin: []byte("no-delimiter\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R2.4: Output delimiter (--output-delimiter) ---
		{
			// AC7: --output-delimiter=, -f1,3 uses comma as output separator.
			Name:  "output_delimiter_comma",
			Args:  []string{"-d:", "--output-delimiter=,", "-f1,3"},
			Stdin: []byte("a:b:c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "output_delimiter_string",
			Args:  []string{"-d:", "--output-delimiter= -> ", "-f1,2"},
			Stdin: []byte("x:y:z\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// --output-delimiter with -b mode.
			Name:  "output_delimiter_bytes",
			Args:  []string{"-b1,3,5", "--output-delimiter=_"},
			Stdin: []byte("abcdef\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- R3.1: Complement mode (--complement) ---
		{
			Name:  "complement_bytes",
			Args:  []string{"--complement", "-b2-4"},
			Stdin: []byte("abcdef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "complement_fields",
			Args:  []string{"--complement", "-d:", "-f2"},
			Stdin: []byte("a:b:c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R3.3: Complement with -f, fields not in list in original order.
			Name:  "complement_fields_multiple",
			Args:  []string{"--complement", "-d:", "-f1,3"},
			Stdin: []byte("a:b:c:d\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- Edge cases ---
		{
			Name:  "empty_input",
			Args:  []string{"-f1"},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "no_trailing_newline",
			Args:  []string{"-f1"},
			Stdin: []byte("a\tb"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "stdin_via_dash",
			Args:  []string{"-f1", "-"},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.1: Successful processing exits 0.
			Name:  "exit_0_on_success",
			Args:  []string{"-b1"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.2: Nonexistent file exits 1.
			Name:     "exit_1_nonexistent_file",
			Args:     []string{"-b1", "/nonexistent/cut_test_file"},
			ExitCode: 1,
			Env:      []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{
				clearStderr,
			},
		},
		{
			// R4.2: Processing continues after error for remaining files.
			Name:     "continues_after_error",
			Args:     []string{"-f1", "/nonexistent/cut_test_file", "-"},
			Stdin:    []byte("x\ty\n"),
			ExitCode: 1,
			Env:      []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{
				clearStderr,
			},
		},
		{
			// Field beyond available fields — produces empty.
			Name:  "field_beyond_available",
			Args:  []string{"-d:", "-f5"},
			Stdin: []byte("a:b:c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// Multiple lines with different field counts.
			Name:  "varied_field_counts",
			Args:  []string{"-d:", "-f2"},
			Stdin: []byte("a:b:c\nx\ny:z\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// clearStderr is a NormalizeFunc that replaces any non-empty output with empty
// bytes. Used for error-path tests where stderr message format differs between
// GNU cut and the Go implementation but exit code and stdout must still match.
func clearStderr(b []byte) []byte {
	return nil
}
