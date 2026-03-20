// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd004-ts R1.1–R1.6, R2.1, R2.2.
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
		// R1.1, R1.2, R1.4: Default format with three lines.
		{
			Name:      "default_format_three_lines",
			Args:      []string{},
			Stdin:     []byte("line1\nline2\nline3\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R1.1: Empty stdin produces no output and exits 0.
		{
			Name:  "default_format_empty_stdin",
			Stdin: []byte(""),
		},
		// R1.1, R1.4: Single line with newline.
		{
			Name:      "single_line",
			Args:      []string{},
			Stdin:     []byte("hello world\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R1.1, R1.2: Multi-line input verifies per-line timestamping.
		{
			Name:      "multi_line_timestamps",
			Args:      []string{},
			Stdin:     []byte("a\nb\nc\nd\ne\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R1.5: Partial last line without trailing newline.
		{
			Name:      "partial_last_line",
			Args:      []string{},
			Stdin:     []byte("full line\npartial"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R1.5: Single partial line (no newline at all).
		{
			Name:      "single_partial_line",
			Args:      []string{},
			Stdin:     []byte("no newline"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.1, R2.2: Custom ISO format.
		{
			Name:      "custom_format_iso",
			Args:      []string{"%Y-%m-%d %H:%M:%S"},
			Stdin:     []byte("line1\nline2\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.1, R2.2: Custom time-only format.
		{
			Name:      "custom_format_time_only",
			Args:      []string{"%H:%M:%S"},
			Stdin:     []byte("test\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.2: Literal percent via %%.
		{
			Name:  "format_literal_percent",
			Args:  []string{"%%"},
			Stdin: []byte("line\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
