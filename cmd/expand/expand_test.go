// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main_test

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gexpand")
	if err != nil {
		t.Skip("reference binary gexpand not found")
	}
	tests := []testutils.DiffTest{
		{
			Name:  "single_tab_at_col1",
			Stdin: []byte("\thello\n"),
		},
		{
			Name:  "tab_after_one_char",
			Stdin: []byte("a\tb\n"),
		},
		{
			Name:  "multiple_consecutive_tabs",
			Stdin: []byte("\t\t\n"),
		},
		{
			Name:  "tabs_at_various_positions",
			Stdin: []byte("ab\tcd\tefgh\ti\n"),
		},
		{
			Name:  "no_tabs_passthrough",
			Stdin: []byte("hello world\n"),
		},
		{
			Name:  "empty_input",
			Stdin: []byte(""),
		},
		{
			Name:  "newline_resets_column",
			Stdin: []byte("abc\td\nabc\td\n"),
		},
		{
			Name:  "tab_at_tab_stop_boundary",
			Stdin: []byte("1234567\t8\n"),
		},
		{
			Name:  "multiple_lines_mixed",
			Stdin: []byte("a\tb\n\tc\nab\tcd\n"),
		},
		{
			Name:  "tab_only",
			Stdin: []byte("\t"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
