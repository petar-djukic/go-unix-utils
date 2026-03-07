// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ts against the moreutils reference binary.
// Implements prd004-ts R9.
package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests comparing the Go ts binary against the
// moreutils reference binary (ts). Tests that produce wall-clock or elapsed
// timestamps use TimestampNormalizer to replace timestamp values with a
// fixed placeholder before comparison. (R9.1, R9.2)
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
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
			// R1.6: empty stdin produces no output and exits 0.
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
			// R2.3, R2.4: subsecond extension %.S.
			Name:      "subsecond_format_dotS",
			Args:      []string{"%.S"},
			Stdin:     []byte("test\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R4.1, R4.2: -s mode with elapsed since start.
			Name:      "elapsed_since_start_mode",
			Args:      []string{"-s"},
			Stdin:     []byte("a\nb\nc\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R3.1, R3.2: -i mode with incremental time.
			Name:      "incremental_mode",
			Args:      []string{"-i"},
			Stdin:     []byte("first\nsecond\nthird\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R5.1, R5.2: -m mode with monotonic clock.
			Name:      "monotonic_clock_mode",
			Args:      []string{"-m"},
			Stdin:     []byte("tick\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R8.1: TZ environment variable is respected.
			Name:      "tz_environment_respected",
			Args:      []string{"%H:%M:%S"},
			Stdin:     []byte("event\n"),
			Env:       []string{"TZ=UTC"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// R6.4: -r mode passes through lines with no recognized timestamp.
			Name:  "relative_mode_no_timestamp_passthrough",
			Args:  []string{"-r"},
			Stdin: []byte("no timestamp here\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestErrorCases verifies error handling behavior of the Go ts binary
// directly, without differential comparison (error messages differ between
// Perl and Go implementations). (R3.4, R6.5, R7.2)
func TestErrorCases(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	tests := []struct {
		name           string
		args           []string
		wantNonZero    bool
		stderrContains string
	}{
		{
			// R7.2: unrecognized flag prints usage to stderr and exits non-zero.
			name:           "invalid_flag",
			args:           []string{"-z"},
			wantNonZero:    true,
			stderrContains: "usage",
		},
		{
			// R3.4: -i and -s are mutually exclusive.
			name:           "incremental_and_elapsed_mutually_exclusive",
			args:           []string{"-i", "-s"},
			wantNonZero:    true,
			stderrContains: "usage",
		},
		{
			// R6.5: -r is mutually exclusive with -i.
			name:           "relative_and_incremental_mutually_exclusive",
			args:           []string{"-r", "-i"},
			wantNonZero:    true,
			stderrContains: "usage",
		},
		{
			// R6.5: -r is mutually exclusive with -s.
			name:           "relative_and_elapsed_mutually_exclusive",
			args:           []string{"-r", "-s"},
			wantNonZero:    true,
			stderrContains: "usage",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := exec.Command(goBin, tc.args...)
			cmd.Stdin = bytes.NewReader([]byte(""))
			var errBuf bytes.Buffer
			cmd.Stderr = &errBuf

			err := cmd.Run()
			if tc.wantNonZero {
				if err == nil {
					t.Errorf("expected non-zero exit code, got 0")
				}
				if !bytes.Contains(errBuf.Bytes(), []byte(tc.stderrContains)) {
					t.Errorf("stderr %q does not contain %q", errBuf.String(), tc.stderrContains)
				}
			}
		})
	}
}
