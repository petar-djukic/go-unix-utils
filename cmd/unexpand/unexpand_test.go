// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunexpand")
	if err != nil {
		t.Skipf("reference binary gunexpand not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		{
			Name:  "8_leading_spaces_to_tab",
			Stdin: []byte("        text\n"),
		},
		{
			Name:  "16_leading_spaces_to_two_tabs",
			Stdin: []byte("                text\n"),
		},
		{
			Name:  "4_leading_spaces_stay",
			Stdin: []byte("    text\n"),
		},
		{
			Name:  "non_leading_spaces_unchanged",
			Stdin: []byte("text        text\n"),
		},
		{
			Name:  "existing_tab_passthrough",
			Stdin: []byte("\ttext\n"),
		},
		{
			Name:  "spaces_then_tab_in_leading",
			Stdin: []byte("   \ttext\n"),
		},
		{
			Name:  "12_leading_spaces_partial",
			Stdin: []byte("            text\n"),
		},
		{
			Name:  "empty_input",
			Stdin: []byte(""),
		},
		{
			Name:  "newline_only",
			Stdin: []byte("\n"),
		},
		{
			Name:  "no_whitespace",
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "tab_then_8_spaces",
			Stdin: []byte("\t        text\n"),
		},
		{
			Name:  "multiple_lines",
			Stdin: []byte("        first\n                second\n    third\n"),
		},
		{
			Name:  "trailing_spaces_unchanged",
			Stdin: []byte("text        \n"),
		},
		{
			Name:  "mixed_leading_tab_then_spaces",
			Stdin: []byte("\t    text\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
