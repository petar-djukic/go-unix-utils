// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gptx")
	if err != nil {
		t.Skipf("reference binary gptx not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		{
			Name:  "single_line",
			Stdin: []byte("the quick brown fox\n"),
		},
		{
			Name:  "empty_input",
			Stdin: []byte(""),
		},
		{
			Name:  "multiple_lines",
			Stdin: []byte("hello world\nfoo bar baz\n"),
		},
		{
			Name:  "ignore_case_flag",
			Args:  []string{"-f"},
			Stdin: []byte("Alpha beta Gamma\n"),
		},
		{
			Name:  "single_word_line",
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "width_flag",
			Args:  []string{"-w", "40"},
			Stdin: []byte("one two three four\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
