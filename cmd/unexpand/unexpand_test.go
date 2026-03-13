// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd025-unexpand R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.1, R3.2, R3.3, R4.1, R4.2, R4.3, R4.4
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests comparing the Go unexpand binary against the
// GNU reference binary (gunexpand) via pkg/testutils.RunDiffTests.
//
// R1.1-R1.4: Default leading-space-to-tab conversion with tab stop 8.
// R2.1-R2.3: -a flag for all-whitespace conversion.
// R3.1-R3.3: -t flag for custom tab stops; -t implies -a.
// R4.1-R4.4: Error handling, exit codes, edge cases, version/help.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunexpand")
	if err != nil {
		t.Skipf("reference binary gunexpand not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// === R1.1-R1.4: Default behavior (leading whitespace only) ===
		{
			// R1.1: Eight leading spaces become one tab.
			Name:  "leading_8_spaces_to_tab",
			Stdin: []byte("        text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: Sixteen leading spaces become two tabs.
			Name:  "leading_16_spaces_to_two_tabs",
			Stdin: []byte("                text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.2: Non-leading spaces are not converted in default mode.
			Name:  "nonleading_spaces_unchanged",
			Stdin: []byte("a        b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.3: Partial run of spaces (fewer than 8) at start of line preserved.
			Name:  "partial_spaces_preserved",
			Stdin: []byte("   text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.3: Five leading spaces do not reach tab stop 8.
			Name:  "five_leading_spaces",
			Stdin: []byte("     text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1, R1.3: Ten leading spaces = one tab (8) + two spaces.
			Name:  "ten_leading_spaces",
			Stdin: []byte("          text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.4: Existing tab in leading whitespace followed by spaces.
			Name:  "existing_tab_then_spaces",
			Stdin: []byte("\t        text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.4: Tab followed by fewer spaces than next tab stop.
			Name:  "tab_then_partial_spaces",
			Stdin: []byte("\t   text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: Empty input passes through unchanged.
			Name:  "empty_input",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: Input with no spaces passes through unchanged.
			Name:  "no_spaces_passthrough",
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.2: Multiple lines, each processed independently.
			Name:  "multiple_lines",
			Stdin: []byte("        a\n        b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: No trailing newline — still converts.
			Name:  "no_trailing_newline",
			Stdin: []byte("        text"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.2: Single space in non-leading position unchanged.
			Name:  "single_nonleading_space",
			Stdin: []byte("a b c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.3: Seven leading spaces — not enough for a tab.
			Name:  "seven_leading_spaces",
			Stdin: []byte("       text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: Exactly eight spaces on a line by themselves.
			Name:  "eight_spaces_only",
			Stdin: []byte("        \n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.2: Stdin via "-" argument.
			Name:  "stdin_via_dash",
			Args:  []string{"-"},
			Stdin: []byte("        text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.2: Mixed: leading spaces on first line, non-leading on second.
			Name:  "mixed_leading_and_nonleading",
			Stdin: []byte("        a\nb        c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.4: Spaces followed by tab in leading whitespace.
			Name:  "spaces_then_tab_in_leading",
			Stdin: []byte("   \ttext\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// === R2.1-R2.3: -a flag (convert all whitespace) ===
		{
			// R2.1: -a converts non-leading spaces to tabs at tab stops.
			Name:  "a_flag_converts_nonleading",
			Args:  []string{"-a"},
			Stdin: []byte("a        b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.1: -a still converts leading spaces.
			Name:  "a_flag_leading_spaces",
			Args:  []string{"-a"},
			Stdin: []byte("        text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.2: Single space that does not reach a tab stop is kept as a space.
			Name:  "a_flag_single_space_kept",
			Args:  []string{"-a"},
			Stdin: []byte("a b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.3: -a processes entire line, not stopping at first non-whitespace.
			Name:  "a_flag_entire_line",
			Args:  []string{"-a"},
			Stdin: []byte("x        y        z\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.1: -a with multiple lines.
			Name:  "a_flag_multiple_lines",
			Args:  []string{"-a"},
			Stdin: []byte("a        b\nc        d\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.2: -a with partial spaces (not reaching tab stop).
			Name:  "a_flag_partial_nonleading",
			Args:  []string{"-a"},
			Stdin: []byte("abcde   f\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.1: --all long form.
			Name:  "all_long_form",
			Args:  []string{"--all"},
			Stdin: []byte("a        b\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// === R3.1-R3.3: -t flag (custom tab stops) ===
		{
			// R3.1: -t 4 sets uniform tab stop interval of 4.
			Name:  "t_uniform_4",
			Args:  []string{"-t", "4"},
			Stdin: []byte("    text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R3.1: -t 4 with 8 spaces = two tabs at width 4.
			Name:  "t_uniform_4_eight_spaces",
			Args:  []string{"-t", "4"},
			Stdin: []byte("        text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R3.3: -t implies -a, so non-leading spaces are converted.
			Name:  "t_uniform_4_nonleading",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a   b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R3.1: -t with comma-separated list of absolute positions.
			Name:  "t_list_4_8_12",
			Args:  []string{"-t", "4,8,12"},
			Stdin: []byte("            text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R3.2: Past last explicit tab stop, spaces are kept as-is.
			Name:  "t_list_past_last_stop",
			Args:  []string{"-t", "4,8,12"},
			Stdin: []byte("                text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R3.1: --tabs=4 long form.
			Name:  "tabs_long_form",
			Args:  []string{"--tabs=4"},
			Stdin: []byte("    text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R3.1: -t4 short form with value attached.
			Name:  "t_short_attached",
			Args:  []string{"-t4"},
			Stdin: []byte("    text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R3.1: -t with non-uniform stops and non-leading spaces.
			Name:  "t_list_nonleading",
			Args:  []string{"-t", "4,8,12"},
			Stdin: []byte("abc     d\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R3.1: -t 2 with various space runs.
			Name:  "t_uniform_2",
			Args:  []string{"-t", "2"},
			Stdin: []byte("  text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R3.3: -t implies -a; partial spaces past last stop kept as spaces.
			Name:  "t_list_mixed_stops",
			Args:  []string{"-t", "3,6"},
			Stdin: []byte("      text      end\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// === R4.1-R4.4: Error handling, exit codes, edge cases ===

		// --- Error conditions (R4.1, R4.2) ---
		{
			// R4.2: Exit 1 on nonexistent file; normalize stderr (format differs).
			Name:     "exit_1_nonexistent_file",
			Args:     []string{"/nonexistent/unexpand_test_file"},
			ExitCode: 1,
			Env:      []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{
				clearStderr,
			},
		},
		{
			// R4.2: Processing continues after error for remaining files.
			Name:     "continues_after_error",
			Args:     []string{"/nonexistent/unexpand_test_file", "-"},
			Stdin:    []byte("        text\n"),
			ExitCode: 1,
			Env:      []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{
				clearStderr,
			},
		},
		{
			// R4.1: Invalid option exits 1.
			Name:     "invalid_option",
			Args:     []string{"-Z"},
			ExitCode: 1,
			Env:      []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{
				clearStderr,
			},
		},

		// --- Edge cases (R4.3) ---
		{
			// R4.3: Empty input produces empty output and exit 0.
			Name:  "empty_stdin",
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.3: Binary data passes through unchanged.
			Name:  "binary_data_passthrough",
			Stdin: []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0x0a},
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.3: Binary data with -a passes through unchanged.
			Name:  "binary_data_passthrough_a",
			Args:  []string{"-a"},
			Stdin: []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0x0a},
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.3: Lines with no convertible spaces pass through.
			Name:  "no_convertible_spaces",
			Stdin: []byte("abcdefghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.3: Line with only tabs passes through unchanged.
			Name:  "only_tabs",
			Stdin: []byte("\t\t\ttext\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.3: Mixed tab/space sequences in leading whitespace.
			Name:  "mixed_tab_space_leading",
			Stdin: []byte("\t    \t  text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.3: Mixed tab/space sequences with -a.
			Name:  "mixed_tab_space_a_flag",
			Args:  []string{"-a"},
			Stdin: []byte("text\t    \tnext\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.3: Single newline produces single newline.
			Name:  "single_newline",
			Stdin: []byte("\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.3: Multiple empty lines.
			Name:  "multiple_empty_lines",
			Stdin: []byte("\n\n\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// --- Version and help (R4.4) ---
		{
			// R4.4: --version prints version info to stdout and exits 0.
			Name: "version_flag",
			Args: []string{"--version"},
			Env:  []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{
				normalizeVersionOutput,
			},
		},
		{
			// R4.4: --help prints usage to stdout and exits 0.
			Name: "help_flag",
			Args: []string{"--help"},
			Env:  []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{
				clearStdout,
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// clearStderr is a NormalizeFunc that replaces any non-empty output with empty
// bytes. Used for error-path tests where stderr message format differs between
// GNU unexpand and the Go implementation but exit code and stdout must still match.
func clearStderr(b []byte) []byte {
	return nil
}

// clearStdout is a NormalizeFunc that replaces any stdout with empty bytes.
// Used for --help tests where help text format differs between implementations
// but exit code must match.
func clearStdout(b []byte) []byte {
	return nil
}

// normalizeVersionOutput replaces version output with a fixed string so both
// GNU and Go implementations compare equal. Both print version info to stdout
// and exit 0; the exact text differs.
func normalizeVersionOutput(b []byte) []byte {
	if len(b) > 0 {
		return []byte("version\n")
	}
	return b
}
