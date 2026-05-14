// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// LC_ALL=C ensures locale-independent comparison for -m (R5.1).
var defaultEnv = []string{"LC_ALL=C"}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skip("reference binary gwc not found")
	}
	tests := []testutils.DiffTest{
		{
			Name:  "default_no_flags",
			Args:  nil,
			Stdin: []byte("foo\nbar baz\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_l_only",
			Args:  []string{"-l"},
			Stdin: []byte("one\ntwo\nthree\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_l_no_trailing_newline",
			Args:  []string{"-l"},
			Stdin: []byte("one\ntwo"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_w_only",
			Args:  []string{"-w"},
			Stdin: []byte("  hello   world  \n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_w_multiple_whitespace_types",
			Args:  []string{"-w"},
			Stdin: []byte("a\tb\nc\rd\fe"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_c_only",
			Args:  []string{"-c"},
			Stdin: []byte("hello\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_m_only_ascii",
			Args:  []string{"-m"},
			Stdin: []byte("hello\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_m_lc_all_c_matches_c",
			Args:  []string{"-m"},
			Stdin: []byte("abc\xc0\xc1xyz\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_c_and_m_together_m_wins",
			Args:  []string{"-c", "-m"},
			Stdin: []byte("hello\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_m_then_c_m_wins",
			Args:  []string{"-m", "-c"},
			Stdin: []byte("hello\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flags_lw_combined",
			Args:  []string{"-lw"},
			Stdin: []byte("hello world\ngoodbye\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flags_lwc_combined",
			Args:  []string{"-lwc"},
			Stdin: []byte("test data\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_order_irrelevant_wl",
			Args:  []string{"-w", "-l"},
			Stdin: []byte("one two\nthree\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "empty_input",
			Args:  nil,
			Stdin: []byte(""),
			Env:   defaultEnv,
		},
		{
			Name:  "empty_input_flag_l",
			Args:  []string{"-l"},
			Stdin: []byte(""),
			Env:   defaultEnv,
		},
		{
			Name:  "empty_input_flag_w",
			Args:  []string{"-w"},
			Stdin: []byte(""),
			Env:   defaultEnv,
		},
		{
			Name:  "empty_input_flag_c",
			Args:  []string{"-c"},
			Stdin: []byte(""),
			Env:   defaultEnv,
		},
		{
			Name:  "empty_input_flag_m",
			Args:  []string{"-m"},
			Stdin: []byte(""),
			Env:   defaultEnv,
		},
		{
			Name:  "binary_input",
			Args:  nil,
			Stdin: []byte{0x00, 0x01, 0xff, 0xfe, 0x0a},
			Env:   defaultEnv,
		},
		{
			Name:  "flag_l_single_newline",
			Args:  []string{"-l"},
			Stdin: []byte("\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_w_only_whitespace",
			Args:  []string{"-w"},
			Stdin: []byte("   \t\n  \n"),
			Env:   defaultEnv,
		},
		{
			Name:  "all_flags_combined",
			Args:  []string{"-l", "-w", "-c"},
			Stdin: []byte("hello world\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flags_lm_combined",
			Args:  []string{"-l", "-m"},
			Stdin: []byte("abc\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flags_wm_combined",
			Args:  []string{"-w", "-m"},
			Stdin: []byte("abc def\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flags_lwm_combined",
			Args:  []string{"-l", "-w", "-m"},
			Stdin: []byte("abc def\nghi\n"),
			Env:   defaultEnv,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
