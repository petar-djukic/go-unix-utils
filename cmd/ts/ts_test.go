// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff verifies cmd/ts against the moreutils reference binary ts.
// Implements prd004-ts R9.1-R9.2.
// R9.1: uses TimestampNormalizer for wall-clock timestamp comparison.
// R9.2: covers default format, custom format, empty stdin, partial last line.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath("ts")
	if err != nil {
		t.Skipf("reference binary ts not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1, R1.2, R1.4: default format with three lines.
		{
			Name:      "default_format_three_lines",
			Stdin:     []byte("line1\nline2\nline3\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R1.1, R1.6: empty stdin produces no output and exits 0.
		{
			Name:     "default_format_empty_stdin",
			Stdin:    []byte(""),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.5: partial last line (no trailing newline) is timestamped.
		{
			Name:      "default_format_partial_last_line",
			Stdin:     []byte("partial"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.1: custom strftime format string.
		{
			Name:      "custom_strftime_format",
			Args:      []string{"%Y-%m-%dT%H:%M:%S"},
			Stdin:     []byte("hello\nworld\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R8.1: TZ=UTC causes timestamps to be in UTC.
		{
			Name:      "tz_environment_respected",
			Args:      []string{"%H:%M:%S"},
			Stdin:     []byte("event\n"),
			Env:       []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
