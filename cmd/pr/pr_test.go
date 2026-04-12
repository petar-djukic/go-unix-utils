// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// prDateRe matches YYYY-MM-DD HH:MM timestamps in pr header lines.
var prDateRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}`)

// normalizePrDate replaces date timestamps with a fixed string so
// differential tests are not sensitive to wall-clock time.
func normalizePrDate(b []byte) []byte {
	return prDateRe.ReplaceAll(b, []byte("YYYY-MM-DD HH:MM"))
}

// prProgRe matches the program name prefix in error messages on stderr.
// R5.1: normalizes "pr: " and "gpr: " (and full paths) to "PROG: ".
var prProgRe = regexp.MustCompile(`(?m)^[^\s:]+: `)

func normalizePrProg(b []byte) []byte {
	return prProgRe.ReplaceAll(b, []byte("PROG: "))
}

// normalizePrErrCase lowercases stderr so Go's "no such file or directory"
// matches GNU's "No such file or directory".
func normalizePrErrCase(b []byte) []byte {
	return bytes.ToLower(b)
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpr")
	if err != nil {
		t.Skipf("reference binary gpr not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:      "default_three_lines",
			Stdin:     []byte("line1\nline2\nline3\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate},
		},
		{
			Name:      "empty_stdin",
			Stdin:     []byte{},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate},
		},
		{
			Name:      "multi_page",
			Stdin:     bytes.Repeat([]byte("line\n"), 60),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate},
		},
		{
			Name:      "two_columns_down",
			Args:      []string{"-2"},
			Stdin:     []byte("a\nb\nc\nd\ne\nf\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate},
		},
		{
			Name:      "two_columns_across",
			Args:      []string{"-2", "-a"},
			Stdin:     []byte("a\nb\nc\nd\ne\nf\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate},
		},
		{
			Name:      "three_columns_down",
			Args:      []string{"-3"},
			Stdin:     []byte("a\nb\nc\nd\ne\nf\ng\nh\ni\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate},
		},
		{
			Name:      "columns_flag",
			Args:      []string{"--columns=4"},
			Stdin:     []byte("a\nb\nc\nd\ne\nf\ng\nh\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate},
		},
		{
			Name:      "two_columns_many_lines",
			Args:      []string{"-2"},
			Stdin:     bytes.Repeat([]byte("line\n"), 120),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate},
		},
		// R4.1: line numbering tests
		{
			Name:      "number_lines_default",
			Args:      []string{"-n"},
			Stdin:     []byte("alpha\nbeta\ngamma\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate},
		},
		{
			Name:      "number_lines_with_empty",
			Args:      []string{"-n"},
			Stdin:     []byte("first\n\nsecond\n\nthird\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate},
		},
		{
			Name:      "number_lines_colon_sep",
			Args:      []string{"-n:"},
			Stdin:     []byte("one\ntwo\nthree\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate},
		},
		{
			Name:      "number_lines_colon_width3",
			Args:      []string{"-n:3"},
			Stdin:     []byte("a\nb\nc\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate},
		},
		{
			Name:      "number_lines_multi_page",
			Args:      []string{"-n", "-l", "20"},
			Stdin:     bytes.Repeat([]byte("line\n"), 30),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate},
		},
		{
			Name:      "number_lines_two_columns",
			Args:      []string{"-n", "-2"},
			Stdin:     []byte("a\nb\nc\nd\ne\nf\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate},
		},
		// R5.1: exit code 0 on success (explicit verification)
		{
			Name:      "success_exit_zero",
			Stdin:     []byte("hello\n"),
			ExitCode:  0,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate},
		},
		// R5.1: exit code 1 on error (nonexistent file)
		{
			Name:      "nonexistent_file_exit_one",
			Args:      []string{"nonexistent_file_xyz_no_such"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate, normalizePrProg, normalizePrErrCase},
		},
		// R5.1: exit code 1 on error (different nonexistent path)
		{
			Name:      "nonexistent_file_alt",
			Args:      []string{"no_such_dir/no_such_file"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate, normalizePrProg, normalizePrErrCase},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
