// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/expand against gexpand (GNU coreutils).
//
// Covers prd024-expand R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3.
package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
func discardAll(data []byte) []byte {
	return nil
}

// normalizeExpandErrors replaces program name prefixes so "gexpand:" and
// "expand:" compare identically.
func normalizeExpandErrors(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	var result [][]byte
	for _, line := range lines {
		if bytes.Contains(line, []byte("--help")) &&
			bytes.Contains(line, []byte("Try")) {
			continue
		}
		colonIdx := bytes.Index(line, []byte(": "))
		if colonIdx <= 0 {
			result = append(result, line)
			continue
		}
		prefix := line[:colonIdx]
		if bytes.ContainsAny(prefix, " \t") {
			result = append(result, line)
			continue
		}
		rest := bytes.ToLower(line[colonIdx:])
		result = append(result, append([]byte("expand"), rest...))
	}
	return bytes.Join(result, []byte("\n"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gexpand")
	if err != nil {
		t.Skip("reference binary gexpand not in PATH")
	}

	// Create temp file for unreadable file test
	errDir := t.TempDir()
	nonexistent := filepath.Join(errDir, "nonexistent.txt")

	errNorm := []testutils.NormalizeFunc{normalizeExpandErrors}

	tests := []testutils.DiffTest{
		// R1.1: default tab expansion at every 8th column
		{
			Name:  "default_single_tab",
			Args:  []string{},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: multiple consecutive tabs advance independently
		{
			Name:  "multiple_consecutive_tabs",
			Args:  []string{},
			Stdin: []byte("\t\tx\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: non-tab characters pass through unchanged
		{
			Name:  "no_tabs_passthrough",
			Args:  []string{},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: newline resets column position
		{
			Name:  "multiline_column_reset",
			Args:  []string{},
			Stdin: []byte("\ta\n\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1, R1.3: tab at various column positions
		{
			Name:  "tab_mid_line",
			Args:  []string{},
			Stdin: []byte("abc\tdef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: tab at column 8 boundary
		{
			Name:  "tab_at_column_8",
			Args:  []string{},
			Stdin: []byte("12345678\tx\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: empty input
		{
			Name:  "empty_input",
			Args:  []string{},
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: multiple newlines
		{
			Name:  "only_newlines",
			Args:  []string{},
			Stdin: []byte("\n\n\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: tab at start of line
		{
			Name:  "tab_at_start",
			Args:  []string{},
			Stdin: []byte("\thello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: input with no trailing newline
		{
			Name:  "no_trailing_newline",
			Args:  []string{},
			Stdin: []byte("a\tb"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: stdin via '-' argument
		{
			Name:  "stdin_dash_arg",
			Args:  []string{"-"},
			Stdin: []byte("x\ty\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R2.1: -t N sets uniform tab stop interval
		{
			Name:  "tab_interval_4",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -t N with different interval
		{
			Name:  "tab_interval_2",
			Args:  []string{"-t", "2"},
			Stdin: []byte("a\tb\tc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: -t LIST sets absolute column positions (comma-separated)
		{
			Name:  "tab_list_comma",
			Args:  []string{"-t", "1,5,9"},
			Stdin: []byte("\ta\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: tab past last explicit stop replaced by single space
		{
			Name:  "tab_past_last_stop",
			Args:  []string{"-t", "4"},
			Stdin: []byte("12345678\t9\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: explicit list with tab past all stops
		{
			Name:  "tab_past_explicit_stops",
			Args:  []string{"-t", "3,6"},
			Stdin: []byte("abcdef\tg\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: list with single value = uniform interval
		{
			Name:  "single_value_list_is_uniform",
			Args:  []string{"-t", "4"},
			Stdin: []byte("\t\tx\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -tN attached form
		{
			Name:  "attached_t_value",
			Args:  []string{"-t4"},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: multiple consecutive tabs with explicit stops
		{
			Name:  "consecutive_tabs_explicit_stops",
			Args:  []string{"-t", "4,8,12"},
			Stdin: []byte("\t\t\tx\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: --tabs=N long form
		{
			Name:  "long_tabs_option",
			Args:  []string{"--tabs=4"},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R3.1: -i only expands leading tabs, embedded tabs pass through
		{
			Name:  "initial_leading_and_embedded",
			Args:  []string{"-i"},
			Stdin: []byte("\thello\tworld\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: --initial long form
		{
			Name:  "initial_long_form",
			Args:  []string{"--initial"},
			Stdin: []byte("\thello\tworld\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: -i with only leading tabs (no embedded)
		{
			Name:  "initial_all_leading_tabs",
			Args:  []string{"-i"},
			Stdin: []byte("\t\thello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: -i with no leading tabs (all embedded)
		{
			Name:  "initial_no_leading_tabs",
			Args:  []string{"-i"},
			Stdin: []byte("a\tb\tc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: -i combined with custom tab stop
		{
			Name:  "initial_with_custom_tabstop",
			Args:  []string{"-i", "-t", "4"},
			Stdin: []byte("\thello\tworld\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: -i multiline resets leading state per line
		{
			Name:  "initial_multiline_reset",
			Args:  []string{"-i"},
			Stdin: []byte("a\tb\n\tc\td\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: -i with spaces before tabs (spaces preserve leading state)
		{
			Name:  "initial_space_before_tab",
			Args:  []string{"-i"},
			Stdin: []byte("  \thello\tworld\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.3: NUL bytes pass through unchanged
		{
			Name:  "nul_byte_passthrough",
			Args:  []string{},
			Stdin: []byte("a\x00b\tc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.4: multiple lines without tabs pass through unchanged
		{
			Name:  "multiple_lines_no_tabs",
			Args:  []string{},
			Stdin: []byte("line one\nline two\nline three\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.3: backspace treated as regular byte (non-goal: no column adjustment)
		{
			Name:  "backspace_as_regular_byte",
			Args:  []string{},
			Stdin: []byte("ab\b\tc\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R4.1: --version exits 0
		{
			Name:      "version",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: --help exits 0
		{
			Name:      "help",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.3: invalid tab stop value exits 1
		{
			Name:      "invalid_tab_stop",
			Args:      []string{"-t", "abc"},
			Stdin:     []byte("a\tb\n"),
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R4.3: zero tab stop value exits 1
		{
			Name:      "zero_tab_stop",
			Args:      []string{"-t", "0"},
			Stdin:     []byte("a\tb\n"),
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R4.3: nonexistent file exits 1
		{
			Name:      "nonexistent_file",
			Args:      []string{nonexistent},
			ExitCode:  1,
			Normalize: errNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
