// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cat against GNU gcat.
// Covers prd006-cat R1.1-R1.5, R2.1-R2.4, R3.1-R3.3, R4.1-R4.9, R5.1-R5.4.
package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes error messages so that differences in binary name
// ("cat" vs "gcat") and Go's os.Open error wrapping ("open file:" prefix) do
// not cause false failures. Exit code and stdout are still compared exactly.
func stderrNormalizer() testutils.NormalizeFunc {
	// Matches "cat: " or "gcat: " at the start of a line.
	binPrefix := regexp.MustCompile(`(?m)^g?cat: `)
	// Matches Go-style "open <path>: " wrapping inside error messages.
	openWrap := regexp.MustCompile(`open [^:]+: `)
	return func(b []byte) []byte {
		b = binPrefix.ReplaceAll(b, []byte("CAT: "))
		b = openWrap.ReplaceAll(b, []byte(""))
		b = bytes.ToLower(b)
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcat")
	if err != nil {
		t.Skipf("reference binary gcat not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		// R1: Default behavior (no flags)
		{
			Name:  "stdin_passthrough",
			Stdin: []byte("hello world\n"),
		},
		{
			Name:  "stdin_dash_arg",
			Args:  []string{"-"},
			Stdin: []byte("from stdin\n"),
		},
		{
			Name:  "empty_stdin",
			Stdin: []byte(""),
		},
		{
			Name:  "no_trailing_newline",
			Stdin: []byte("no newline"),
		},
		// R2: Line numbering (-n, -b)
		{
			Name:  "number_all_lines",
			Args:  []string{"-n"},
			Stdin: []byte("alpha\n\nbeta\n"),
		},
		{
			Name:  "number_nonblank",
			Args:  []string{"-b"},
			Stdin: []byte("alpha\n\nbeta\n"),
		},
		{
			Name:  "b_overrides_n",
			Args:  []string{"-n", "-b"},
			Stdin: []byte("first\n\nsecond\n\nthird\n"),
		},
		{
			Name:  "number_blank_definition",
			Args:  []string{"-n"},
			Stdin: []byte("text\n \n\t\n\n"),
		},
		// R3: Blank squeezing (-s)
		{
			Name:  "squeeze_blank",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\n\n\nb\n\n\nc\n"),
		},
		{
			Name:  "squeeze_with_number",
			Args:  []string{"-s", "-n"},
			Stdin: []byte("a\n\n\nb\n"),
		},
		{
			Name:  "squeeze_with_number_nonblank",
			Args:  []string{"-s", "-b"},
			Stdin: []byte("a\n\n\nb\n"),
		},
		// R4: Non-printing display (-v, -E, -T, -A, -e, -t)
		{
			Name:  "show_ends",
			Args:  []string{"-E"},
			Stdin: []byte("line1\nline2\n"),
		},
		{
			Name:  "show_tabs",
			Args:  []string{"-T"},
			Stdin: []byte("col1\tcol2\n"),
		},
		{
			Name:  "show_nonprinting",
			Args:  []string{"-v"},
			Stdin: []byte{0x01, 0x09, 0x0A, 0x1B, 0x7F, 0x80, 0xA0, 0xFE, 0xFF, 0x0A},
		},
		{
			Name:  "show_all_A",
			Args:  []string{"-A"},
			Stdin: []byte("hello\tworld\n\x01\x7f\n"),
		},
		{
			Name:  "show_e_flag",
			Args:  []string{"-e"},
			Stdin: []byte("line\t1\n\x01end\n"),
		},
		{
			Name:  "show_t_flag",
			Args:  []string{"-t"},
			Stdin: []byte("col\t1\n\x01end\n"),
		},
		{
			Name:  "u_flag_ignored",
			Args:  []string{"-u"},
			Stdin: []byte("unchanged\n"),
		},
		// R4.9 / R5: Combined flags and interactions
		{
			Name:  "n_with_show_ends",
			Args:  []string{"-nE"},
			Stdin: []byte("alpha\nbeta\n"),
		},
		{
			Name:  "b_with_squeeze_and_show_ends",
			Args:  []string{"-bsE"},
			Stdin: []byte("first\n\n\n\nsecond\n"),
		},
		{
			Name:  "all_transform_flags",
			Args:  []string{"-n", "-s", "-A"},
			Stdin: []byte("a\x01\tb\n\n\nc\n"),
		},
		// R5: Error handling
		{
			Name:      "nonexistent_file",
			Args:      []string{"nonexistent_file_that_does_not_exist"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		{
			Name:      "nonexistent_mixed_with_stdin",
			Args:      []string{"-", "nonexistent_file_that_does_not_exist"},
			Stdin:     []byte("valid input\n"),
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
