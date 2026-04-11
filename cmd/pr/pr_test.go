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
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
