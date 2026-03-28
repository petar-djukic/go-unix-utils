// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cut against GNU gcut.
// Covers prd026-cut R4.1-R4.4 (exit codes and differential testing).
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes error messages between GNU gcut and Go cut.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?cut|gcut`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	noSuch := regexp.MustCompile(`(?i)no such file or directory`)
	// GNU uses specific messages like "invalid byte/character position"
	// while Go uses a generic "invalid byte, character or field list".
	invalidRange := regexp.MustCompile(
		`invalid (byte/character position|byte, character or field list|field value) '[^']*'`)
	genericRange := regexp.MustCompile(
		`invalid byte, character or field list`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("cut"))
		b = tryHelp.ReplaceAll(b, nil)
		b = noSuch.ReplaceAll(b, []byte("No such file or directory"))
		b = invalidRange.ReplaceAll(b, []byte("invalid range"))
		b = genericRange.ReplaceAll(b, []byte("invalid range"))
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcut")
	if err != nil {
		t.Skipf("reference binary gcut not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		// --- R4.1: Byte mode (-b) ---

		// R1.1: single byte position.
		{
			Name:  "byte_single_position",
			Args:  []string{"-b", "3"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.1: byte range N-M.
		{
			Name:  "byte_range",
			Args:  []string{"-b", "2-4"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.1: open range N- (from N to end).
		{
			Name:  "byte_open_range_start",
			Args:  []string{"-b", "3-"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.1: open range -M (from start to M).
		{
			Name:  "byte_open_range_end",
			Args:  []string{"-b", "-4"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.1: comma-separated list of positions.
		{
			Name:  "byte_comma_list",
			Args:  []string{"-b", "1,3,5"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.4: line shorter than selected range.
		{
			Name:  "byte_short_line",
			Args:  []string{"-b", "5-10"},
			Stdin: []byte("abc\n"),
		},
		// R3.1: --complement with -b inverts selection.
		{
			Name:  "byte_complement",
			Args:  []string{"-b", "2,4", "--complement"},
			Stdin: []byte("abcdef\n"),
		},

		// --- R4.2: Character mode (-c) ---

		// R1.2: -c single position (equivalent to -b under LC_ALL=C).
		{
			Name:  "char_single_position",
			Args:  []string{"-c", "2"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.2: -c range.
		{
			Name:  "char_range",
			Args:  []string{"-c", "2-4"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.2: -c open range N-.
		{
			Name:  "char_open_range_start",
			Args:  []string{"-c", "4-"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.2: -c open range -M.
		{
			Name:  "char_open_range_end",
			Args:  []string{"-c", "-3"},
			Stdin: []byte("abcdef\n"),
		},
		// R1.2: -c comma-separated list.
		{
			Name:  "char_comma_list",
			Args:  []string{"-c", "1,3,6"},
			Stdin: []byte("abcdef\n"),
		},
		// R3.1: --complement with -c.
		{
			Name:  "char_complement",
			Args:  []string{"-c", "1,3,5", "--complement"},
			Stdin: []byte("abcdef\n"),
		},

		// --- R4.3: Field mode (-f) ---

		// R2.1/R2.2: field with delimiter.
		{
			Name:  "field_with_delimiter",
			Args:  []string{"-d:", "-f2"},
			Stdin: []byte("a:b:c\n"),
		},
		// R2.1: field range.
		{
			Name:  "field_range",
			Args:  []string{"-d:", "-f1-2"},
			Stdin: []byte("a:b:c:d\n"),
		},
		// R2.1: multiple fields comma-separated.
		{
			Name:  "field_comma_list",
			Args:  []string{"-d:", "-f1,3"},
			Stdin: []byte("a:b:c\n"),
		},
		// R2.1: open range N- selects from field N to end.
		{
			Name:  "field_open_range_start",
			Args:  []string{"-d:", "-f2-"},
			Stdin: []byte("a:b:c:d\n"),
		},
		// R2.3: -s suppresses lines without delimiter.
		{
			Name:  "field_suppress_no_delim",
			Args:  []string{"-d:", "-f2", "-s"},
			Stdin: []byte("no-delimiter\na:b\n"),
		},
		// R2.3: without -s, lines without delimiter pass through.
		{
			Name:  "field_no_delim_passthrough",
			Args:  []string{"-d:", "-f2"},
			Stdin: []byte("no-delimiter\n"),
		},
		// R2.4: --output-delimiter replaces separator in output.
		{
			Name:  "field_output_delimiter",
			Args:  []string{"-d:", "-f1,3", "--output-delimiter=|"},
			Stdin: []byte("a:b:c\n"),
		},
		// R2.4: --output-delimiter with multi-char string.
		{
			Name:  "field_output_delimiter_multi",
			Args:  []string{"-d:", "-f1,2,3", "--output-delimiter=, "},
			Stdin: []byte("a:b:c\n"),
		},
		// R2.2: tab as default delimiter.
		{
			Name:  "field_tab_delimiter",
			Args:  []string{"-f2"},
			Stdin: []byte("a\tb\tc\n"),
		},
		// R3.1/R3.3: --complement with -f inverts field selection.
		{
			Name:  "field_complement",
			Args:  []string{"-d:", "--complement", "-f2"},
			Stdin: []byte("a:b:c\n"),
		},
		// R3.3: --complement with -f and output-delimiter.
		{
			Name:  "field_complement_output_delim",
			Args:  []string{"-d:", "--complement", "-f2", "--output-delimiter=|"},
			Stdin: []byte("a:b:c:d\n"),
		},

		// --- R4.4: Error cases ---

		// No mode flag specified — exit 1.
		{
			Name:      "error_no_mode",
			Args:      []string{},
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// Invalid range — exit 1.
		{
			Name:      "error_invalid_range",
			Args:      []string{"-b", "abc"},
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// Multi-byte delimiter — exit 1.
		{
			Name:      "error_multi_byte_delim",
			Args:      []string{"-d", "ab", "-f1"},
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// Nonexistent file — exit 1.
		{
			Name:      "error_nonexistent_file",
			Args:      []string{"-b", "1", "/nonexistent-path/no-such-file.txt"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},

		// --- Additional coverage ---

		// R1.3: multiple lines from stdin.
		{
			Name:  "byte_multi_line",
			Args:  []string{"-b", "1-3"},
			Stdin: []byte("abcdef\nxyz123\n"),
		},
		// R1.1: overlapping ranges merged.
		{
			Name:  "byte_overlapping_ranges",
			Args:  []string{"-b", "1-3,2-5"},
			Stdin: []byte("abcdefgh\n"),
		},
		// Empty stdin.
		{
			Name:  "empty_stdin",
			Args:  []string{"-b", "1"},
			Stdin: []byte{},
		},
		// Stdin via - argument.
		{
			Name:  "stdin_dash",
			Args:  []string{"-b", "1-3", "-"},
			Stdin: []byte("abcdef\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
