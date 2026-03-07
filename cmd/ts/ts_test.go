// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ts against ts reference binary (moreutils).
// Implements prd004-ts R1-R5, R7-R8.
package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// subsecondNormalizer replaces SS.USEC patterns (e.g., "41.191463") that
// TimestampNormalizer does not cover.
var subsecondNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`\d{2}\.\d{6}`)
	return re.ReplaceAll(b, []byte("<SUBSEC>"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ts")
	if err != nil {
		t.Skipf("reference binary ts not in PATH: %v", err)
	}

	tsNorm := []testutils.NormalizeFunc{testutils.TimestampNormalizer}

	tests := []testutils.DiffTest{
		// R1.1, R1.2, R1.4: Default format with three-line stdin.
		{
			Name:      "default_format_three_lines",
			Args:      []string{},
			Stdin:     []byte("line1\nline2\nline3\n"),
			Normalize: tsNorm,
		},
		// R1.6: Empty stdin produces no output.
		{
			Name:  "default_format_empty_stdin",
			Args:  []string{},
			Stdin: []byte(""),
		},
		// R1.5: Partial last line (no trailing newline) is timestamped.
		{
			Name:      "default_format_partial_last_line",
			Args:      []string{},
			Stdin:     []byte("partial"),
			Normalize: tsNorm,
		},
		// R2.1, R2.2: Custom strftime format string.
		{
			Name:      "custom_strftime_format",
			Args:      []string{"%Y-%m-%dT%H:%M:%S"},
			Stdin:     []byte("hello\nworld\n"),
			Normalize: tsNorm,
		},
		// R2.3: Subsecond extension %.S.
		{
			Name:      "subsecond_format_dotS",
			Args:      []string{"%.S"},
			Stdin:     []byte("test\n"),
			Normalize: []testutils.NormalizeFunc{subsecondNormalizer},
		},
		// R4.1, R4.2: -s mode elapsed since start.
		{
			Name:      "elapsed_since_start_mode",
			Args:      []string{"-s"},
			Stdin:     []byte("a\nb\nc\n"),
			Normalize: tsNorm,
		},
		// R3.1, R3.2: -i mode incremental timestamps.
		{
			Name:      "incremental_mode",
			Args:      []string{"-i"},
			Stdin:     []byte("first\nsecond\nthird\n"),
			Normalize: tsNorm,
		},
		// R5.1: -m mode with default format.
		{
			Name:      "monotonic_clock_mode",
			Args:      []string{"-m"},
			Stdin:     []byte("tick\n"),
			Normalize: tsNorm,
		},
		// R8.1: TZ=UTC respected in wall-clock timestamps.
		{
			Name:      "tz_environment_respected",
			Args:      []string{"%H:%M:%S"},
			Stdin:     []byte("event\n"),
			Env:       []string{"TZ=UTC"},
			Normalize: tsNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestErrorCases tests error handling independently since the reference ts (Perl)
// uses different exit codes (255) and error message formats than our Go implementation.
// R3.4, R7.2.
func TestErrorCases(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	t.Run("incremental_and_elapsed_mutually_exclusive", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(goBin, "-i", "-s")
		cmd.Stdin = bytes.NewReader([]byte(""))
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected non-zero exit, got 0")
		}
		if !strings.Contains(stderr.String(), "usage") {
			t.Errorf("expected stderr to contain 'usage', got: %q", stderr.String())
		}
	})

	t.Run("invalid_flag", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(goBin, "-z")
		cmd.Stdin = bytes.NewReader([]byte(""))
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected non-zero exit, got 0")
		}
		if !strings.Contains(stderr.String(), "usage") {
			t.Errorf("expected stderr to contain 'usage', got: %q", stderr.String())
		}
	})
}
