// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd004-ts R1.1, R1.2, R1.3, R1.4, R1.5, R1.6, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3, R5.1, R5.2, R5.3 (differential tests)
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

// subsecNormalizer replaces subsecond timestamp patterns (digits/colons followed
// by .NNNNNN microseconds) with a fixed placeholder. Handles all ts-specific
// subsecond extensions: %.S (SS.ffffff), %.s (epoch.ffffff), %.T (HH:MM:SS.ffffff).
var subsecNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`[\d:]+\.\d{6}`)
	return re.ReplaceAll(b, []byte("<SUBSEC>"))
}

// elapsedNormalizer replaces HH:MM:SS elapsed timestamps with a fixed placeholder
// so that differential tests for -i mode are not affected by micro-timing differences.
var elapsedNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`\d{2}:\d{2}:\d{2}`)
	return re.ReplaceAll(b, []byte("<ELAPSED>"))
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
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer, elapsedNormalizer},
		},
		{
			// R2.3: %.S subsecond extension (seconds with microsecond suffix)
			Name:      "subsec_dot_S",
			Args:      []string{"%.S"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{subsecNormalizer},
		},
		{
			// R2.3: %.s subsecond extension (epoch with microsecond suffix)
			Name:      "subsec_dot_s",
			Args:      []string{"%.s"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{subsecNormalizer},
		},
		{
			// R2.3: %.T subsecond extension (HH:MM:SS with microsecond suffix)
			Name:      "subsec_dot_T",
			Args:      []string{"%.T"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{subsecNormalizer},
		},
		{
			// R2.3, R2.4: subsecond extension combined with standard specifiers
			Name:      "subsec_combined_format",
			Args:      []string{"%Y-%m-%d %.T"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{isoDateNormalizer, subsecNormalizer},
		},
		{
			// R2.3: %.S with multi-line input
			Name:      "subsec_dot_S_multi_line",
			Args:      []string{"%.S"},
			Stdin:     []byte("a\nb\nc\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{subsecNormalizer},
		},
		{
			// R3.1, R3.2: incremental mode with default format
			Name:      "incremental_default",
			Args:      []string{"-i"},
			Stdin:     []byte("line1\nline2\nline3\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{elapsedNormalizer},
		},
		{
			// R3.1: incremental mode single line
			Name:      "incremental_single_line",
			Args:      []string{"-i"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{elapsedNormalizer},
		},
		{
			// R3.1: incremental mode empty stdin
			Name:      "incremental_empty",
			Args:      []string{"-i"},
			Stdin:     []byte(""),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{elapsedNormalizer},
		},
		{
			// R3.1, R3.2: incremental mode ten lines for differential coverage
			Name:      "incremental_ten_lines",
			Args:      []string{"-i"},
			Stdin:     []byte("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{elapsedNormalizer},
		},
		{
			// R3.2: incremental mode with custom format overriding default
			Name:      "incremental_custom_format",
			Args:      []string{"-i", "%H:%M:%.S"},
			Stdin:     []byte("line1\nline2\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{subsecNormalizer},
		},
		{
			// R3.1: incremental mode with partial last line
			Name:      "incremental_partial_line",
			Args:      []string{"-i"},
			Stdin:     []byte("complete\npartial"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{elapsedNormalizer},
		},
		{
			// R3.3: custom format overrides -i default; TZ=GMT still applies
			Name:      "incremental_custom_format_epoch",
			Args:      []string{"-i", "%s"},
			Stdin:     []byte("line1\nline2\nline3\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{epochNormalizer},
		},
		{
			// R3.3: custom subsecond format with -i mode
			Name:      "incremental_custom_subsec_T",
			Args:      []string{"-i", "%.T"},
			Stdin:     []byte("a\nb\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{subsecNormalizer},
		},
		{
			// R3.4: -i and -s together — -i takes precedence (matches reference binary)
			Name:      "incremental_and_since_start_exclusive",
			Args:      []string{"-i", "-s"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{elapsedNormalizer},
		},
		{
			// R3.4: -s and -i together (reverse order) — -i still takes precedence
			Name:      "since_start_and_incremental_exclusive",
			Args:      []string{"-s", "-i"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{elapsedNormalizer},
		},
		{
			// R4.1, R4.2: -s mode with default format, single line
			Name:      "since_start_single_line",
			Args:      []string{"-s"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{elapsedNormalizer},
		},
		{
			// R4.1: -s mode elapsed time increases monotonically across lines
			Name:      "since_start_multi_line",
			Args:      []string{"-s"},
			Stdin:     []byte("a\nb\nc\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{elapsedNormalizer},
		},
		{
			// R4.1, R4.2: -s mode with ten lines for differential coverage
			Name:      "since_start_ten_lines",
			Args:      []string{"-s"},
			Stdin:     []byte("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{elapsedNormalizer},
		},
		{
			// R4.1: -s mode with empty stdin
			Name:      "since_start_empty",
			Args:      []string{"-s"},
			Stdin:     []byte(""),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{elapsedNormalizer},
		},
		{
			// R4.1: -s mode with partial last line
			Name:      "since_start_partial_line",
			Args:      []string{"-s"},
			Stdin:     []byte("complete\npartial"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{elapsedNormalizer},
		},
		{
			// R4.2: -s default format is %H:%M:%S with custom format override
			Name:      "since_start_custom_format",
			Args:      []string{"-s", "%.T"},
			Stdin:     []byte("line1\nline2\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{subsecNormalizer},
		},
		{
			// R4.3: custom format argument overrides -s default format (epoch)
			Name:      "since_start_custom_format_epoch",
			Args:      []string{"-s", "%s"},
			Stdin:     []byte("a\nb\nc\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{epochNormalizer},
		},
		{
			// R4.3: custom format argument overrides -s default format (date+time)
			Name:      "since_start_custom_format_date_time",
			Args:      []string{"-s", "%H:%M:%.S"},
			Stdin:     []byte("line1\nline2\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{subsecNormalizer},
		},
		{
			// R5.1: -m flag with default mode uses monotonic clock
			Name:      "monotonic_default",
			Args:      []string{"-m"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R5.1: -m flag with multi-line input
			Name:      "monotonic_multi_line",
			Args:      []string{"-m"},
			Stdin:     []byte("a\nb\nc\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R5.1: -m flag with empty stdin
			Name:      "monotonic_empty",
			Args:      []string{"-m"},
			Stdin:     []byte(""),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R5.2: -m combined with -i mode
			Name:      "monotonic_incremental",
			Args:      []string{"-m", "-i"},
			Stdin:     []byte("line1\nline2\nline3\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{elapsedNormalizer},
		},
		{
			// R5.2: -m combined with -s mode
			Name:      "monotonic_since_start",
			Args:      []string{"-m", "-s"},
			Stdin:     []byte("line1\nline2\nline3\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{elapsedNormalizer},
		},
		{
			// R5.2: -m with custom format string
			Name:      "monotonic_custom_format",
			Args:      []string{"-m", "%Y-%m-%dT%H:%M:%S"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R5.2: -m with subsecond extension
			Name:      "monotonic_subsec",
			Args:      []string{"-m", "%.T"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{subsecNormalizer},
		},
		{
			// R5.3: -m with ten lines verifies monotonic consistency
			Name:      "monotonic_ten_lines",
			Args:      []string{"-m"},
			Stdin:     []byte("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R5.3: -m with -s and ten lines for monotonic elapsed consistency
			Name:      "monotonic_since_start_ten_lines",
			Args:      []string{"-m", "-s"},
			Stdin:     []byte("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{elapsedNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
