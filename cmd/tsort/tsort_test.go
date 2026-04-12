// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/tsort against gtsort.
// Implements srd102-tsort R2.1-R2.3 acceptance criteria via testutils.RunDiffTests.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeStderr replaces the reference binary name and normalizes
// error message casing so differential comparison succeeds.
func normalizeStderr(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("gtsort:"), []byte("tsort:"))
	lines := bytes.Split(data, []byte("\n"))
	for i, line := range lines {
		idx := bytes.Index(line, []byte("/tsort:"))
		if idx >= 0 && len(line) > 0 && line[0] == '/' {
			lines[i] = append([]byte("tsort:"), line[idx+len("/tsort:"):]...)
		}
	}
	data = bytes.Join(lines, []byte("\n"))
	data = bytes.ReplaceAll(data,
		[]byte("No such file or directory"),
		[]byte("no such file or directory"))
	data = bytes.ReplaceAll(data,
		[]byte("Permission denied"),
		[]byte("permission denied"))
	data = bytes.ReplaceAll(data,
		[]byte("Is a directory"),
		[]byte("is a directory"))
	return data
}

// normalizeStderrHint strips the "Try '...' for more information." line
// that GNU tsort appends to some error messages.
func normalizeStderrHint(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	var out [][]byte
	for _, l := range lines {
		if bytes.HasPrefix(l, []byte("Try '")) {
			continue
		}
		out = append(out, l)
	}
	return bytes.Join(out, []byte("\n"))
}

// clearOutput returns nil, used to ignore stdout/stderr content
// and compare only exit codes (e.g., for --help/--version).
func clearOutput(data []byte) []byte {
	return nil
}

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestDiff runs differential tests for tsort against gtsort.
// Covers srd102-tsort R1.1-R1.4, R2.1-R2.3 acceptance criteria.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtsort")
	if err != nil {
		t.Skipf("reference binary gtsort not in PATH: %v", err)
	}

	dir := t.TempDir()
	linearFile := writeTestFile(t, dir, "linear.txt", "a b b c c d\n")
	cycleFile := writeTestFile(t, dir, "cycle.txt", "a b b a\n")
	selfFile := writeTestFile(t, dir, "self.txt", "a a\n")
	emptyFile := writeTestFile(t, dir, "empty.txt", "")
	oddFile := writeTestFile(t, dir, "odd.txt", "a b c\n")
	tabFile := writeTestFile(t, dir, "tabs.txt", "a\tb\tb\tc\n")
	multiLineFile := writeTestFile(t, dir, "multi.txt", "a b\nb c\nc d\n")

	// Create a permission-denied file for R2.2 testing.
	noReadFile := writeTestFile(t, dir, "noread.txt", "a b\n")
	os.Chmod(noReadFile, 0o000)
	t.Cleanup(func() { os.Chmod(noReadFile, 0o644) })

	stderrNorm := []testutils.NormalizeFunc{normalizeStderr, normalizeStderrHint}

	tests := []testutils.DiffTest{
		// --- R1.1: basic topological sort ---
		{
			Name:  "linear_chain_stdin",
			Stdin: []byte("a b b c c d\n"),
		},
		{
			Name: "linear_chain_file",
			Args: []string{linearFile},
		},
		{
			Name:  "single_pair",
			Stdin: []byte("a b\n"),
		},
		{
			Name:  "diamond_graph",
			Stdin: []byte("a b a c b d c d\n"),
		},
		{
			Name:  "self_pair",
			Stdin: []byte("a a\n"),
		},
		{
			Name: "self_pair_file",
			Args: []string{selfFile},
		},
		{
			Name:  "disconnected_nodes",
			Stdin: []byte("a a b b c c\n"),
		},
		// R1.1: tab-separated tokens
		{
			Name: "tab_separated_file",
			Args: []string{tabFile},
		},
		// R1.1: multi-line input with one pair per line
		{
			Name: "multi_line_file",
			Args: []string{multiLineFile},
		},
		// R1.1: tokens with mixed whitespace via stdin
		{
			Name:  "mixed_whitespace_stdin",
			Stdin: []byte("a\tb\n  b\tc\n"),
		},

		// --- R1.2: cycle detection ---
		{
			Name:      "cycle_two_nodes",
			Stdin:     []byte("a b b a\n"),
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			Name:      "cycle_file",
			Args:      []string{cycleFile},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			Name:      "cycle_three_nodes",
			Stdin:     []byte("a b b c c a\n"),
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			Name:      "cycle_with_linear",
			Stdin:     []byte("a b b c c b\n"),
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// R1.1: self-pair does not create a cycle edge
		{
			Name:  "self_pair_no_cycle",
			Stdin: []byte("a b b b\n"),
		},
		// R1.2: multiple independent cycles
		{
			Name:      "two_independent_cycles",
			Stdin:     []byte("a b b a c d d c\n"),
			ExitCode:  1,
			Normalize: stderrNorm,
		},

		// --- R1.3: odd number of tokens ---
		{
			Name:      "odd_tokens_stdin",
			Stdin:     []byte("a b c\n"),
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			Name:      "odd_tokens_file",
			Args:      []string{oddFile},
			ExitCode:  1,
			Normalize: stderrNorm,
		},

		// --- R1.4: file input modes ---
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("a b b c\n"),
		},
		{
			Name:  "empty_stdin",
			Stdin: []byte(""),
		},
		{
			Name: "empty_file",
			Args: []string{emptyFile},
		},

		// --- R2.1: invalid argument errors ---
		{
			Name:      "extra_operand",
			Args:      []string{linearFile, cycleFile},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// R2.1: invalid short option
		{
			Name:      "invalid_short_option",
			Args:      []string{"-x"},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// R2.1: invalid long option
		{
			Name:      "invalid_long_option",
			Args:      []string{"--invalid"},
			ExitCode:  1,
			Normalize: stderrNorm,
		},

		// --- R2.2: file error handling ---
		{
			Name:      "nonexistent_file",
			Args:      []string{filepath.Join(dir, "no_such_file.txt")},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// R2.2: permission denied on file
		{
			Name:      "permission_denied",
			Args:      []string{noReadFile},
			ExitCode:  1,
			Normalize: stderrNorm,
		},

		// --- R2.1: --help and --version exit 0 ---
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},

		// --- R1.4: -- separator handling ---
		{
			Name: "double_dash_separator",
			Args: []string{"--", linearFile},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
