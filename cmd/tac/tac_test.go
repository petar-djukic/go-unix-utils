// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tac. Implements srd021-tac R4.1, R4.2, R4.3.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtac")
	if err != nil {
		t.Skipf("reference binary gtac not in PATH: %v", err)
	}

	// Create temp files for file-based tests.
	dir := t.TempDir()

	fileA := filepath.Join(dir, "a.txt")
	os.WriteFile(fileA, []byte("a\nb\nc\n"), 0o644)

	fileB := filepath.Join(dir, "b.txt")
	os.WriteFile(fileB, []byte("x\ny\n"), 0o644)

	noNewline := filepath.Join(dir, "nonl.txt")
	os.WriteFile(noNewline, []byte("a\nb\nc"), 0o644)

	colonFile := filepath.Join(dir, "colon.txt")
	os.WriteFile(colonFile, []byte("a:b:c:"), 0o644)

	colonBefore := filepath.Join(dir, "colonbefore.txt")
	os.WriteFile(colonBefore, []byte(":a:b:c"), 0o644)

	// R4.2: tests cover single-file reversal, stdin reversal, multi-file
	// reversal, -s with a custom separator, -b flag, and no trailing newline.
	// R4.3: LC_ALL=C is set by default via testutils.RunDiffTests buildEnv.
	tests := []testutils.DiffTest{
		// R4.2: single-file reversal.
		{
			Name: "single_file_reversal",
			Args: []string{fileA},
		},
		// R4.2: stdin reversal.
		{
			Name: "stdin_reversal",
			Stdin: []byte("a\nb\nc\n"),
		},
		// R4.2: stdin via explicit dash.
		{
			Name: "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("one\ntwo\nthree\n"),
		},
		// R4.2: multi-file reversal (each file reversed independently).
		{
			Name: "multi_file_reversal",
			Args: []string{fileA, fileB},
		},
		// R4.2: file with no trailing newline.
		{
			Name: "no_trailing_newline",
			Args: []string{noNewline},
		},
		// R4.2: -s with a custom separator.
		{
			Name: "custom_separator_colon",
			Args: []string{"-s", ":", colonFile},
		},
		// R4.2: -b flag (separator before record).
		{
			Name: "before_flag_colon",
			Args: []string{"-b", "-s", ":", colonBefore},
		},
		// Combined -b -s via stdin.
		{
			Name: "before_flag_stdin",
			Args:  []string{"-b", "-s", ":"},
			Stdin: []byte(":a:b:c"),
		},
		// Custom separator via stdin.
		{
			Name: "custom_sep_stdin",
			Args:  []string{"-s", ":"},
			Stdin: []byte("a:b:c:"),
		},
		// Single line, no reversal needed.
		{
			Name: "single_line",
			Stdin: []byte("hello\n"),
		},
		// Empty input.
		{
			Name: "empty_input",
			Stdin: []byte{},
		},
		// Multi-char separator.
		{
			Name: "multi_char_separator",
			Args:  []string{"-s", "::"},
			Stdin: []byte("a::b::c::"),
		},
	}

	// R4.1: compare Go tac output against gtac byte-for-byte.
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
