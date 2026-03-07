// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests against the Homebrew moreutils ts binary.
// Uses testutils.TimestampNormalizer to mask wall-clock differences (D4, R9.1).
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	// D3: reference binary is "ts" (moreutils does not use g-prefix).
	refBin, err := exec.LookPath("ts")
	if err != nil {
		t.Skipf("reference binary ts not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R1.1, R1.2, R1.4: default format with three-line stdin.
			Name:      "default_format_three_lines",
			Args:      []string{},
			Stdin:     []byte("line1\nline2\nline3\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R1.6: empty stdin produces no output, exit 0.
			Name:  "default_format_empty_stdin",
			Args:  []string{},
			Stdin: []byte(""),
		},
		{
			// R1.5: partial last line (no trailing newline) is timestamped.
			Name:      "default_format_partial_last_line",
			Args:      []string{},
			Stdin:     []byte("partial"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R2.1, R2.2: custom strftime format string.
			Name:      "custom_strftime_format",
			Args:      []string{"%Y-%m-%dT%H:%M:%S"},
			Stdin:     []byte("hello\nworld\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R4.1, R4.2: -s mode, elapsed since start.
			Name:      "elapsed_since_start_mode",
			Args:      []string{"-s"},
			Stdin:     []byte("a\nb\nc\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R3.1, R3.2: -i mode, incremental per-line elapsed.
			Name:      "incremental_mode",
			Args:      []string{"-i"},
			Stdin:     []byte("first\nsecond\nthird\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R5.1, R5.2: -m mode, monotonic clock with default format.
			Name:      "monotonic_clock_mode",
			Args:      []string{"-m"},
			Stdin:     []byte("tick\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R8.1: TZ environment variable respected.
			Name:      "tz_environment_respected",
			Args:      []string{"%H:%M:%S"},
			Stdin:     []byte("event\n"),
			Env:       []string{"TZ=UTC"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R5: -m with -s mode.
			Name:      "monotonic_with_elapsed",
			Args:      []string{"-m", "-s"},
			Stdin:     []byte("x\ny\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestInvalidFlag verifies that an unrecognized flag produces a usage message
// on stderr and exits non-zero (R7.2). This test is non-differential because
// error message format differs between Go flag package and Perl Getopt.
func TestInvalidFlag(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "-z")
	cmd.Stdin = bytes.NewReader(nil)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit for invalid flag -z")
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "usage") {
		t.Errorf("stderr should contain 'usage', got: %q", stderr.String())
	}
}
