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
		{
			Name:  "t_uniform_4",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a\tb\n"),
		},
		{
			Name:  "t_uniform_4_attached",
			Args:  []string{"-t4"},
			Stdin: []byte("a\tb\n"),
		},
		{
			Name:  "t_uniform_2",
			Args:  []string{"-t", "2"},
			Stdin: []byte("\t\t\n"),
		},
		{
			Name:  "t_uniform_at_boundary",
			Args:  []string{"-t", "4"},
			Stdin: []byte("abc\td\n"),
		},
		{
			Name:  "t_uniform_multiple_tabs",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a\tb\tc\n"),
		},
		{
			Name:  "t_list_absolute",
			Args:  []string{"-t", "1,5,9"},
			Stdin: []byte("a\tb\tc\n"),
		},
		{
			Name:  "t_list_past_last_stop",
			Args:  []string{"-t", "4,8"},
			Stdin: []byte("abcdefgh\tx\n"),
		},
		{
			Name:  "t_list_two_stops",
			Args:  []string{"-t", "3,6"},
			Stdin: []byte("a\tb\tc\n"),
		},
		{
			Name:  "t_list_single_value_is_uniform",
			Args:  []string{"-t", "4"},
			Stdin: []byte("ab\tcd\tefgh\ti\n"),
		},
		{
			Name:  "t_multiple_flags_concatenate",
			Args:  []string{"-t", "2", "-t", "4"},
			Stdin: []byte("a\tb\n"),
		},
		{
			Name:  "t_uniform_no_tabs",
			Args:  []string{"-t", "4"},
			Stdin: []byte("hello world\n"),
		},
		{
			Name:  "t_list_consecutive_tabs",
			Args:  []string{"-t", "4,8,12"},
			Stdin: []byte("\t\t\t\n"),
		},
		{
			Name:  "t_list_tab_at_col1",
			Args:  []string{"-t", "5,10"},
			Stdin: []byte("\thello\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
