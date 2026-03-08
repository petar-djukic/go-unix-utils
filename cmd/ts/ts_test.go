// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// subsecondNormalizer replaces subsecond-precision numbers
// (e.g., "32.001234", "1708358732.001234") with a fixed placeholder.
var subsecondNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`\d+\.\d{6}`)
	return re.ReplaceAll(b, []byte("<SUBSEC>"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ts")
	if err != nil {
		t.Skipf("reference binary ts not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R1.1, R1.2, R1.4: Default format with three-line stdin.
			Name:      "default_format_three_lines",
			Args:      []string{},
			Stdin:     []byte("line1\nline2\nline3\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R1.6: Empty stdin produces no output.
			Name:  "default_format_empty_stdin",
			Stdin: []byte(""),
		},
		{
			// R1.5: Partial last line (no trailing newline).
			Name:      "default_format_partial_last_line",
			Args:      []string{},
			Stdin:     []byte("partial"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R2.1, R2.2: Custom strftime format string.
			Name:      "custom_strftime_format",
			Args:      []string{"%Y-%m-%dT%H:%M:%S"},
			Stdin:     []byte("hello\nworld\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R2.3: Subsecond extension %.S.
			Name:      "subsecond_format_dotS",
			Args:      []string{"%.S"},
			Stdin:     []byte("test\n"),
			Normalize: []testutils.NormalizeFunc{subsecondNormalizer},
		},
		{
			// R4.1, R4.2: Elapsed-since-start mode.
			Name:      "elapsed_since_start_mode",
			Args:      []string{"-s"},
			Stdin:     []byte("a\nb\nc\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R3.1, R3.2: Incremental mode.
			Name:      "incremental_mode",
			Args:      []string{"-i"},
			Stdin:     []byte("first\nsecond\nthird\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R5.1, R5.2: Monotonic clock mode with default format.
			Name:      "monotonic_clock_mode",
			Args:      []string{"-m"},
			Stdin:     []byte("tick\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R6.4: -r mode passthrough for lines with no timestamp.
			Name:  "relative_mode_no_timestamp_passthrough",
			Args:  []string{"-r"},
			Stdin: []byte("no timestamp here\n"),
		},
		{
			// R8.1: TZ environment is respected.
			Name:      "tz_environment_respected",
			Args:      []string{"%H:%M:%S"},
			Stdin:     []byte("event\n"),
			Env:       []string{"TZ=UTC"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// Reference binary accepts -i -s without error; -i takes precedence.
			Name:      "incremental_and_elapsed_together",
			Args:      []string{"-i", "-s"},
			Stdin:     []byte("test\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R7.2: Invalid flag exits 255 with usage message.
			Name:     "invalid_flag",
			Args:     []string{"-z"},
			Stdin:    []byte(""),
			ExitCode: 255,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
