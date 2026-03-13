// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd004-ts R1.1, R1.2, R1.3, R1.4 (differential tests)
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ts")
	if err != nil {
		t.Skipf("reference binary ts not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R1.1, R1.2: single line with default timestamp format
			Name:      "single_line_default_format",
			Stdin:     []byte("hello world\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R1.1: multiple lines each get a timestamp
			Name:      "multi_line",
			Stdin:     []byte("line1\nline2\nline3\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R1.4, R1.1: empty stdin produces no output, exit 0
			Name:      "empty_stdin",
			Stdin:     []byte(""),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R1.1: blank line still gets a timestamp
			Name:      "blank_lines",
			Stdin:     []byte("\n\n\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R1.1, R1.2: ten-line input for differential coverage
			Name:      "ten_lines",
			Stdin:     []byte("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
