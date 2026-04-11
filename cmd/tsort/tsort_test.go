// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/tsort against gtsort.
// Implements srd102-tsort acceptance criteria via testutils.RunDiffTests.
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
// D2: uses testutils.BuildBinary and exec.LookPath with t.Skip.
// D4: LC_ALL=C is set by default via testutils.
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

	stderrNorm := []testutils.NormalizeFunc{normalizeStderr, normalizeStderrHint}

	tests := []testutils.DiffTest{
		// AC1: basic linear chain via stdin
		{
			Name:  "linear_chain_stdin",
			Stdin: []byte("a b b c c d\n"),
		},
		// AC1: linear chain from file argument
		{
			Name: "linear_chain_file",
			Args: []string{linearFile},
		},
		// AC1: reading from stdin via "-"
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("a b b c\n"),
		},
		// AC2: diamond graph ordering
		{
			Name:  "diamond_graph",
			Stdin: []byte("a b a c b d c d\n"),
		},
		// R1.1: self pair registers node without creating edge
		{
			Name:  "self_pair",
			Stdin: []byte("a a\n"),
		},
		// R1.4: self pair from file
		{
			Name: "self_pair_file",
			Args: []string{selfFile},
		},
		// AC3: cycle detection with two nodes
		{
			Name:      "cycle_two_nodes",
			Stdin:     []byte("a b b a\n"),
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// AC3: cycle detection from file
		{
			Name:      "cycle_file",
			Args:      []string{cycleFile},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// AC3: three-node cycle
		{
			Name:      "cycle_three_nodes",
			Stdin:     []byte("a b b c c a\n"),
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// R1.1: empty input produces empty output
		{
			Name:  "empty_stdin",
			Stdin: []byte(""),
		},
		// R1.4: empty file
		{
			Name: "empty_file",
			Args: []string{emptyFile},
		},
		// R1.3: odd number of tokens
		{
			Name:      "odd_tokens_stdin",
			Stdin:     []byte("a b c\n"),
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// R1.3: odd tokens from file
		{
			Name:      "odd_tokens_file",
			Args:      []string{oddFile},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// R1.1: single pair
		{
			Name:  "single_pair",
			Stdin: []byte("a b\n"),
		},
		// R1.1: multiple disconnected self-pairs
		{
			Name:  "disconnected_nodes",
			Stdin: []byte("a a b b c c\n"),
		},
		// AC4: --help prints usage and exits 0
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// AC4: --version prints version and exits 0
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// R1.4: extra operand error
		{
			Name:      "extra_operand",
			Args:      []string{linearFile, cycleFile},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// R1.4: nonexistent file error
		{
			Name:      "nonexistent_file",
			Args:      []string{filepath.Join(dir, "no_such_file.txt")},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// R1.2: cycle mixed with linear dependencies
		{
			Name:      "cycle_with_linear",
			Stdin:     []byte("a b b c c b\n"),
			ExitCode:  1,
			Normalize: stderrNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
