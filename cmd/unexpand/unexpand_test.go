// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/unexpand against gunexpand (GNU coreutils).
//
// Covers prd025-unexpand R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3.
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

// normalizeUnexpandErrors replaces program name prefixes so "gunexpand:" and
// "unexpand:" compare identically.
func normalizeUnexpandErrors(data []byte) []byte {
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
		result = append(result, append([]byte("unexpand"), rest...))
	}
	return bytes.Join(result, []byte("\n"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunexpand")
	if err != nil {
		t.Skip("reference binary gunexpand not in PATH")
	}

	errDir := t.TempDir()
	nonexistent := filepath.Join(errDir, "nonexistent.txt")
	errNorm := []testutils.NormalizeFunc{normalizeUnexpandErrors}

	tests := []testutils.DiffTest{
		// R1.1: 8 leading spaces become one tab
		{
			Name:  "leading_8_spaces_to_tab",
			Args:  []string{},
			Stdin: []byte("        text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: 16 leading spaces become two tabs
		{
			Name:  "leading_16_spaces_to_two_tabs",
			Args:  []string{},
			Stdin: []byte("                text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: non-leading spaces unchanged in default mode
		{
			Name:  "non_leading_spaces_unchanged",
			Args:  []string{},
			Stdin: []byte("a        b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: 4 leading spaces (not reaching tab stop) kept as spaces
		{
			Name:  "partial_spaces_kept",
			Args:  []string{},
			Stdin: []byte("    text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: 10 spaces = 1 tab + 2 remaining spaces
		{
			Name:  "spaces_partial_after_tab_stop",
			Args:  []string{},
			Stdin: []byte("          text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: existing tab in leading whitespace preserved
		{
			Name:  "existing_tab_leading",
			Args:  []string{},
			Stdin: []byte("\ttext\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: spaces before tab absorbed into tab
		{
			Name:  "spaces_before_tab_absorbed",
			Args:  []string{},
			Stdin: []byte("   \ttext\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: tab followed by 4 spaces (partial, kept as spaces)
		{
			Name:  "tab_then_partial_spaces",
			Args:  []string{},
			Stdin: []byte("\t    text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: tab followed by 8 spaces becomes two tabs
		{
			Name:  "tab_then_8_spaces",
			Args:  []string{},
			Stdin: []byte("\t        text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: multiple leading tabs unchanged
		{
			Name:  "multiple_leading_tabs",
			Args:  []string{},
			Stdin: []byte("\t\ttext\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: empty input
		{
			Name:  "empty_input",
			Args:  []string{},
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: only newlines
		{
			Name:  "only_newlines",
			Args:  []string{},
			Stdin: []byte("\n\n\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: no whitespace at all
		{
			Name:  "no_whitespace",
			Args:  []string{},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1, R1.3: multiline with different leading widths
		{
			Name:  "multiline_mixed",
			Args:  []string{},
			Stdin: []byte("        line1\n    line2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: no trailing newline
		{
			Name:  "no_trailing_newline",
			Args:  []string{},
			Stdin: []byte("        text"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: stdin via '-' argument
		{
			Name:  "stdin_dash_arg",
			Args:  []string{"-"},
			Stdin: []byte("        text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: single leading space (less than tab stop)
		{
			Name:  "single_leading_space",
			Args:  []string{},
			Stdin: []byte(" text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: non-leading tab passes through unchanged
		{
			Name:  "non_leading_tab_passthrough",
			Args:  []string{},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1, R1.2: mixed line with leading and non-leading spaces
		{
			Name:  "leading_converted_nonleading_kept",
			Args:  []string{},
			Stdin: []byte("        hello        world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Nonexistent file exits 1
		{
			Name:      "nonexistent_file",
			Args:      []string{nonexistent},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// --version exits 0
		{
			Name:      "version",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// --help exits 0
		{
			Name:      "help",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},

		// ---- R2.1: -a converts all runs of spaces to tabs at tab stops ----
		// R2.1: non-leading 8 spaces become tab with -a
		{
			Name:  "a_non_leading_8_spaces",
			Args:  []string{"-a"},
			Stdin: []byte("a        b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -a still converts leading spaces
		{
			Name:  "a_leading_spaces",
			Args:  []string{"-a"},
			Stdin: []byte("        text\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -a with multiple runs of spaces on one line
		{
			Name:  "a_multiple_space_runs",
			Args:  []string{"-a"},
			Stdin: []byte("a        b        c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: --all long form
		{
			Name:  "all_long_form",
			Args:  []string{"--all"},
			Stdin: []byte("a        b\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// ---- R2.2: single space kept with -a ----
		// R2.2: single non-leading space not reaching tab stop
		{
			Name:  "a_single_space_kept",
			Args:  []string{"-a"},
			Stdin: []byte("ab cd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: 3 spaces that don't reach a tab stop
		{
			Name:  "a_partial_spaces_kept",
			Args:  []string{"-a"},
			Stdin: []byte("abcde   f\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// ---- R2.3: -a processes entire line, not just leading ----
		// R2.3: text before and after spaces all converted
		{
			Name:  "a_entire_line_processed",
			Args:  []string{"-a"},
			Stdin: []byte("hello        world        end\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: tabs in non-leading position with -a
		{
			Name:  "a_non_leading_tab",
			Args:  []string{"-a"},
			Stdin: []byte("a\t        b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: multiline with -a
		{
			Name:  "a_multiline",
			Args:  []string{"-a"},
			Stdin: []byte("a        b\nc        d\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: empty input with -a
		{
			Name:  "a_empty_input",
			Args:  []string{"-a"},
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: only spaces with -a (not reaching tab stop)
		{
			Name:  "a_only_spaces_no_tabstop",
			Args:  []string{"-a"},
			Stdin: []byte("abc   \n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
