// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tac against gtac reference binary.
// Implements prd021-tac R4.
package main

import (
	"os/exec"
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

	tests := []testutils.DiffTest{
		// R1.1, R1.2: Default reversal with trailing newline.
		{
			Name:  "default_reversal",
			Args:  []string{},
			Stdin: []byte("alpha\nbeta\ngamma\n"),
		},
		// R1.2: No trailing newline preserved.
		{
			Name:  "no_trailing_newline",
			Args:  []string{},
			Stdin: []byte("a\nb\nc"),
		},
		// R1.1: Single line reverses to itself.
		{
			Name:  "single_line",
			Args:  []string{},
			Stdin: []byte("only\n"),
		},
		// R1.3: Empty input produces empty output.
		{
			Name:  "empty_input",
			Args:  []string{},
			Stdin: []byte(""),
		},
		// R2.1: Custom separator -s.
		{
			Name:  "custom_separator",
			Args:  []string{"-s", ":"},
			Stdin: []byte("a:b:c:"),
		},
		// R2.2: -b places separator before record.
		{
			Name:  "before_flag",
			Args:  []string{"-b", "-s", ":"},
			Stdin: []byte(":a:b:c"),
		},
		// R2.3: -r interprets separator as regex.
		{
			Name:  "regex_separator",
			Args:  []string{"-r", "-s", "[0-9]+"},
			Stdin: []byte("a1b2c3"),
		},
		// R2.4: -r combined with -b.
		{
			Name:  "regex_before",
			Args:  []string{"-r", "-b", "-s", "[0-9]+"},
			Stdin: []byte("1a2b3c"),
		},
		// Multi-character separator.
		{
			Name:  "multi_char_separator",
			Args:  []string{"-s", "::"},
			Stdin: []byte("x::y::z::"),
		},
		// R1.2: Two lines with trailing newline.
		{
			Name:  "two_lines",
			Args:  []string{},
			Stdin: []byte("first\nsecond\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
