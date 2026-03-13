// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd004-ts R1.1, R1.2, R1.3, R1.4, R1.5, R1.6, R2.1, R2.2 (differential tests)
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// isoDateNormalizer replaces YYYY-MM-DD date strings with a fixed placeholder
// so that differential tests for date-only custom formats are not affected by
// day boundaries between Go and reference binary execution.
var isoDateNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	return re.ReplaceAll(b, []byte("<DATE>"))
}

// epochNormalizer replaces Unix epoch seconds with a fixed placeholder so that
// differential tests using %s are not affected by timing differences.
var epochNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`\d{10,}`)
	return re.ReplaceAll(b, []byte("<EPOCH>"))
}

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
		{
			// R1.5: partial last line without trailing newline
			Name:      "partial_last_line",
			Stdin:     []byte("no newline at end"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R1.5: mixed complete and partial lines
			Name:      "mixed_complete_and_partial",
			Stdin:     []byte("complete\npartial"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R1.6: exit 0 on clean EOF with single line
			Name:      "exit_zero_on_eof",
			Stdin:     []byte("line\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R2.1: custom ISO 8601 format string
			Name:      "custom_format_iso8601",
			Args:      []string{"%Y-%m-%dT%H:%M:%S"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R2.1: custom format with date only
			Name:      "custom_format_date_only",
			Args:      []string{"%Y-%m-%d"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{isoDateNormalizer},
		},
		{
			// R2.2: custom format with multiple strftime specifiers
			Name:      "custom_format_weekday_month",
			Args:      []string{"%A %B %d"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer, isoDateNormalizer},
		},
		{
			// R2.2: epoch seconds format
			Name:      "custom_format_epoch",
			Args:      []string{"%s"},
			Stdin:     []byte("epoch\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{epochNormalizer},
		},
		{
			// R2.1, R2.2: custom format with multi-line input
			Name:      "custom_format_multi_line",
			Args:      []string{"%Y-%m-%dT%H:%M:%S"},
			Stdin:     []byte("a\nb\nc\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R2.2: literal percent in format
			Name:      "custom_format_literal_percent",
			Args:      []string{"%%-%H:%M:%S"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
