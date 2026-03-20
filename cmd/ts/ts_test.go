// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd004-ts R1.1–R1.6, R2.1–R2.4, R3.1–R3.4, R4.1–R4.2.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// subsecNormalizer replaces bare decimal number patterns (e.g. "21.924603")
// that appear in %.S output but are not matched by TimestampNormalizer.
var subsecNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	re := regexp.MustCompile(`\d+\.\d+`)
	return re.ReplaceAll(data, []byte("<TIMESTAMP>"))
}

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
		// R2.3: %.S subsecond extension (seconds with microseconds).
		{
			Name:      "subsecond_dot_S",
			Args:      []string{"%.S"},
			Stdin:     []byte("test\n"),
			Normalize: []testutils.NormalizeFunc{subsecNormalizer},
		},
		// R2.3: %.s subsecond extension (epoch with microseconds).
		{
			Name:      "subsecond_dot_s",
			Args:      []string{"%.s"},
			Stdin:     []byte("test\n"),
			Normalize: []testutils.NormalizeFunc{subsecNormalizer},
		},
		// R2.3: %.T subsecond extension (HH:MM:SS with microseconds).
		{
			Name:      "subsecond_dot_T",
			Args:      []string{"%.T"},
			Stdin:     []byte("test\n"),
			Normalize: []testutils.NormalizeFunc{subsecNormalizer},
		},
		// R2.3, R2.4: Custom format mixing standard and subsecond extensions.
		{
			Name:      "subsecond_mixed_format",
			Args:      []string{"%Y-%m-%d %.T"},
			Stdin:     []byte("test\n"),
			Normalize: []testutils.NormalizeFunc{subsecNormalizer},
		},
		// R3.1, R3.2: Incremental mode with default format.
		{
			Name:      "incremental_default_format",
			Args:      []string{"-i"},
			Stdin:     []byte("line1\nline2\nline3\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R3.1: Incremental mode with single line.
		{
			Name:      "incremental_single_line",
			Args:      []string{"-i"},
			Stdin:     []byte("only\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R3.1, R3.2: Incremental mode with custom subsecond format.
		{
			Name:      "incremental_custom_format",
			Args:      []string{"-i", "%.T"},
			Stdin:     []byte("a\nb\n"),
			Normalize: []testutils.NormalizeFunc{subsecNormalizer},
		},
		// R3.1: Incremental mode with empty stdin.
		{
			Name:  "incremental_empty_stdin",
			Args:  []string{"-i"},
			Stdin: []byte(""),
		},
		// R3.3: Custom format overrides -i default; TZ=GMT still applies.
		{
			Name:      "incremental_custom_hm_format",
			Args:      []string{"-i", "%H:%M"},
			Stdin:     []byte("line1\nline2\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R4.1, R4.2: Elapsed-since-start mode with default format.
		{
			Name:      "elapsed_default_format",
			Args:      []string{"-s"},
			Stdin:     []byte("line1\nline2\nline3\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R4.1: Elapsed mode with single line.
		{
			Name:      "elapsed_single_line",
			Args:      []string{"-s"},
			Stdin:     []byte("only\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R4.1: Elapsed mode with empty stdin.
		{
			Name:  "elapsed_empty_stdin",
			Args:  []string{"-s"},
			Stdin: []byte(""),
		},
		// R4.1: Elapsed mode with custom subsecond format.
		{
			Name:      "elapsed_custom_subsecond",
			Args:      []string{"-s", "%.T"},
			Stdin:     []byte("a\nb\n"),
			Normalize: []testutils.NormalizeFunc{subsecNormalizer},
		},
		// R4.1: Elapsed mode with partial last line.
		{
			Name:      "elapsed_partial_last_line",
			Args:      []string{"-s"},
			Stdin:     []byte("full\npartial"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestMutualExclusivity verifies R3.4: -i and -s together produce an error.
// This is a standalone test because the Perl reference binary does not enforce
// this constraint, so differential testing cannot verify it.
func TestMutualExclusivity(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "-i", "-s")
	cmd.Stdin = nil
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for -i -s, got 0; output: %s", out)
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() == 0 {
		t.Fatalf("expected non-zero exit code, got 0")
	}
}
