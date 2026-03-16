// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ts against ts (moreutils).
// Implements prd004-ts R1.1-R1.4, R9.1-R9.2 test coverage.
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
		// R1.1: single line with default format.
		{
			Name:      "R1.1_single_line_default_format",
			Stdin:     []byte("hello world\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R1.1: multi-line stdin.
		{
			Name:      "R1.1_multi_line",
			Stdin:     []byte("line one\nline two\nline three\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R1.2: default format produces "Mon DD HH:MM:SS" pattern.
		{
			Name:      "R1.2_default_format",
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R1.3/R2.1: custom strftime format.
		{
			Name:      "R2.1_custom_format_iso",
			Args:      []string{"%Y-%m-%d %H:%M:%S"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R9.2: empty stdin produces no output and exits 0.
		{
			Name:      "R9.2_empty_stdin",
			Stdin:     []byte(""),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R9.2: 10-line input for differential test.
		{
			Name:      "R9.2_ten_lines",
			Stdin:     []byte("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R1.1: line with spaces and special characters.
		{
			Name:      "R1.1_special_chars",
			Stdin:     []byte("hello\tworld\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.1: custom format with date only.
		{
			Name:      "R2.1_custom_format_date_only",
			Args:      []string{"%Y-%m-%d"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
