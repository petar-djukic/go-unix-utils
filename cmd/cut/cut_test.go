// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cut against gcut (GNU coreutils).
// Covers prd026-cut R1.1–R1.4: byte and character selection.
// Covers prd026-cut R2.1–R2.4: field selection, delimiter, suppress, output delimiter.
// Covers prd026-cut R3.1–R3.3: complement mode for bytes, characters, and fields.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcut")
	if err != nil {
		t.Skip("reference binary gcut not in PATH")
	}
	tests := buildByteTests()
	tests = append(tests, buildFieldTests()...)
	tests = append(tests, buildComplementTests()...)
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// buildByteTests returns differential tests for R1.1–R1.4.
func buildByteTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "byte_single_position",
			Args:  []string{"-b", "2"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "byte_range",
			Args:  []string{"-b", "2-4"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "byte_open_start",
			Args:  []string{"-b", "-3"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "byte_open_end",
			Args:  []string{"-b", "4-"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "byte_comma_list",
			Args:  []string{"-b", "1,3,5"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "byte_mixed_ranges",
			Args:  []string{"-b", "1-2,5-6"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "byte_overlapping_ranges",
			Args:  []string{"-b", "1-4,3-6"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "char_single_position",
			Args:  []string{"-c", "2"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "char_range",
			Args:  []string{"-c", "2-4"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "short_line_out_of_range",
			Args:  []string{"-b", "5-10"},
			Stdin: []byte("abc\n"),
		},
		{
			Name:  "very_short_line_all_out_of_range",
			Args:  []string{"-b", "10-20"},
			Stdin: []byte("abc\n"),
		},
		{
			Name:  "multiple_lines",
			Args:  []string{"-b", "2-3"},
			Stdin: []byte("abcdef\nghijkl\n"),
		},
		{
			Name:  "empty_line",
			Args:  []string{"-b", "1"},
			Stdin: []byte("\n"),
		},
		{
			Name:  "byte_flag_attached",
			Args:  []string{"-b1-3"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "char_flag_attached",
			Args:  []string{"-c1,4"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "stdin_dash",
			Args:  []string{"-b", "1-3", "-"},
			Stdin: []byte("abcdef\n"),
		},
	}
}

// buildFieldTests returns differential tests for R2.1–R2.4.
func buildFieldTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		// R2.1: -f LIST field extraction with tab delimiter (default)
		{
			Name:  "field_tab_default_single",
			Args:  []string{"-f", "2"},
			Stdin: []byte("a\tb\tc\n"),
		},
		{
			Name:  "field_tab_default_range",
			Args:  []string{"-f", "1-2"},
			Stdin: []byte("a\tb\tc\n"),
		},
		{
			Name:  "field_tab_default_comma_list",
			Args:  []string{"-f", "1,3"},
			Stdin: []byte("a\tb\tc\n"),
		},
		{
			Name:  "field_tab_open_end",
			Args:  []string{"-f", "2-"},
			Stdin: []byte("a\tb\tc\td\n"),
		},
		{
			Name:  "field_tab_open_start",
			Args:  []string{"-f", "-2"},
			Stdin: []byte("a\tb\tc\n"),
		},
		// R2.2: -d DELIM custom delimiter
		{
			Name:  "field_colon_delim",
			Args:  []string{"-d:", "-f", "2"},
			Stdin: []byte("a:b:c\n"),
		},
		{
			Name:  "field_colon_delim_multi_field",
			Args:  []string{"-d:", "-f", "1,3"},
			Stdin: []byte("a:b:c\n"),
		},
		{
			Name:  "field_pipe_delim",
			Args:  []string{"-d|", "-f", "2"},
			Stdin: []byte("a|b|c\n"),
		},
		{
			Name:  "field_space_delim",
			Args:  []string{"-d", " ", "-f", "2"},
			Stdin: []byte("a b c\n"),
		},
		{
			Name:  "field_delim_long_form",
			Args:  []string{"--delimiter=:", "-f", "2"},
			Stdin: []byte("a:b:c\n"),
		},
		// R2.1: out-of-range fields produce nothing
		{
			Name:  "field_out_of_range",
			Args:  []string{"-d:", "-f", "5"},
			Stdin: []byte("a:b:c\n"),
		},
		// R2.3: -s suppress lines without delimiter
		{
			Name:  "field_suppress_no_delim",
			Args:  []string{"-d:", "-f", "2", "-s"},
			Stdin: []byte("no-delimiter\n"),
		},
		{
			Name:  "field_suppress_with_delim",
			Args:  []string{"-d:", "-f", "2", "-s"},
			Stdin: []byte("a:b:c\n"),
		},
		{
			Name:  "field_suppress_mixed_lines",
			Args:  []string{"-d:", "-f", "1", "-s"},
			Stdin: []byte("a:b:c\nno-delim\nx:y:z\n"),
		},
		{
			Name:  "field_no_suppress_no_delim",
			Args:  []string{"-d:", "-f", "2"},
			Stdin: []byte("no-delimiter\n"),
		},
		{
			Name:  "field_suppress_long_form",
			Args:  []string{"-d:", "-f", "1", "--only-delimited"},
			Stdin: []byte("no-delim\na:b\n"),
		},
		// R2.4: --output-delimiter
		{
			Name:  "field_output_delim",
			Args:  []string{"-d:", "-f", "1,3", "--output-delimiter=|"},
			Stdin: []byte("a:b:c\n"),
		},
		{
			Name:  "field_output_delim_multi_char",
			Args:  []string{"-d:", "-f", "1,2,3", "--output-delimiter=, "},
			Stdin: []byte("a:b:c\n"),
		},
		{
			Name:  "field_output_delim_range",
			Args:  []string{"-d:", "-f", "1-3", "--output-delimiter= -> "},
			Stdin: []byte("a:b:c\n"),
		},
		{
			Name:  "field_output_delim_empty",
			Args:  []string{"-d:", "-f", "1,3", "--output-delimiter=_"},
			Stdin: []byte("a:b:c\n"),
		},
		// R2.1: multiple lines with fields
		{
			Name:  "field_multiple_lines",
			Args:  []string{"-d:", "-f", "2"},
			Stdin: []byte("a:b:c\nx:y:z\n"),
		},
		// R2.1: -f with attached value
		{
			Name:  "field_flag_attached",
			Args:  []string{"-d:", "-f2"},
			Stdin: []byte("a:b:c\n"),
		},
		// R2.1: fields long form
		{
			Name:  "field_long_form",
			Args:  []string{"-d:", "--fields=1,3"},
			Stdin: []byte("a:b:c\n"),
		},
	}
}

// buildComplementTests returns differential tests for R3.1–R3.3.
func buildComplementTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		// R3.1: --complement with -b inverts byte selection
		{
			Name:  "complement_bytes_single",
			Args:  []string{"-b", "2", "--complement"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "complement_bytes_range",
			Args:  []string{"-b", "2-4", "--complement"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "complement_bytes_open_start",
			Args:  []string{"-b", "-3", "--complement"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "complement_bytes_open_end",
			Args:  []string{"-b", "4-", "--complement"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "complement_bytes_comma_list",
			Args:  []string{"-b", "1,3,5", "--complement"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "complement_bytes_multiple_lines",
			Args:  []string{"-b", "2-3", "--complement"},
			Stdin: []byte("abcdef\nghijkl\n"),
		},
		// R3.2: --complement with -c
		{
			Name:  "complement_chars_range",
			Args:  []string{"-c", "2-4", "--complement"},
			Stdin: []byte("abcdef\n"),
		},
		// R3.1, R3.3: --complement with -f inverts field selection
		{
			Name:  "complement_field_single",
			Args:  []string{"-d:", "-f", "2", "--complement"},
			Stdin: []byte("a:b:c\n"),
		},
		{
			Name:  "complement_field_range",
			Args:  []string{"-d:", "-f", "1-2", "--complement"},
			Stdin: []byte("a:b:c:d\n"),
		},
		{
			Name:  "complement_field_comma_list",
			Args:  []string{"-d:", "-f", "1,3", "--complement"},
			Stdin: []byte("a:b:c:d\n"),
		},
		{
			Name:  "complement_field_all_selected",
			Args:  []string{"-d:", "-f", "1-3", "--complement"},
			Stdin: []byte("a:b:c\n"),
		},
		// R3.3: --complement with -f and --output-delimiter
		{
			Name:  "complement_field_output_delim",
			Args:  []string{"-d:", "-f", "2", "--complement", "--output-delimiter=|"},
			Stdin: []byte("a:b:c\n"),
		},
		// R3.1: --complement with -f, line without delimiter passes through
		{
			Name:  "complement_field_no_delim_line",
			Args:  []string{"-d:", "-f", "2", "--complement"},
			Stdin: []byte("no-delimiter\n"),
		},
		// R3.2: --complement with -f on multiple lines
		{
			Name:  "complement_field_multiple_lines",
			Args:  []string{"-d:", "-f", "2", "--complement"},
			Stdin: []byte("a:b:c\nx:y:z\n"),
		},
		// R3.1: --complement with -b on empty line
		{
			Name:  "complement_bytes_empty_line",
			Args:  []string{"-b", "1", "--complement"},
			Stdin: []byte("\n"),
		},
		// R3.1: --complement with -b short line
		{
			Name:  "complement_bytes_short_line",
			Args:  []string{"-b", "5-10", "--complement"},
			Stdin: []byte("abc\n"),
		},
	}
}
