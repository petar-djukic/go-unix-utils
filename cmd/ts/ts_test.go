// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ts against ts (moreutils).
// Implements prd004-ts R1.1-R1.6, R2.1-R2.4, R3.1-R3.4, R4.1-R4.3, R9.1-R9.2 test coverage.
package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// epochSubsecNormalizer replaces Unix epoch timestamps with microsecond suffix
// (e.g., "1708358732.001234") with a fixed placeholder. Used for %.s tests.
var epochSubsecNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`\d{9,}\.\d{6}`)
	return re.ReplaceAll(b, []byte("<EPOCH_USEC>"))
}

// deltaSubsecNormalizer replaces any number with microsecond suffix (e.g.,
// "0.000005") with a fixed placeholder. Used for %.s tests in -i/-s modes
// where the epoch value is a small delta rather than a wall-clock epoch.
var deltaSubsecNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`\d+\.\d{6}`)
	return re.ReplaceAll(b, []byte("<DELTA_USEC>"))
}

// secSubsecNormalizer replaces seconds with microsecond suffix (e.g.,
// "32.001234") with a fixed placeholder. Used for %.S tests.
var secSubsecNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`\d{2}\.\d{6}`)
	return re.ReplaceAll(b, []byte("<SEC_USEC>"))
}

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
		// R3.1: incremental mode single line.
		{
			Name:      "R3.1_incremental_single_line",
			Args:      []string{"-i"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R3.1: incremental mode multi-line.
		{
			Name:      "R3.1_incremental_multi_line",
			Args:      []string{"-i"},
			Stdin:     []byte("line one\nline two\nline three\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R3.3: incremental mode with custom format.
		{
			Name:      "R3.3_incremental_custom_format",
			Args:      []string{"-i", "%H:%M:%S"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R4.1: elapsed mode single line.
		{
			Name:      "R4.1_elapsed_single_line",
			Args:      []string{"-s"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R4.1: elapsed mode multi-line.
		{
			Name:      "R4.1_elapsed_multi_line",
			Args:      []string{"-s"},
			Stdin:     []byte("line one\nline two\nline three\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R4.3: elapsed mode with custom format.
		{
			Name:      "R4.3_elapsed_custom_format",
			Args:      []string{"-s", "%H:%M:%S"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R9.2: incremental mode empty stdin.
		{
			Name:      "R9.2_incremental_empty_stdin",
			Args:      []string{"-i"},
			Stdin:     []byte(""),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R9.2: elapsed mode empty stdin.
		{
			Name:      "R9.2_elapsed_empty_stdin",
			Args:      []string{"-s"},
			Stdin:     []byte(""),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.1: custom format with time-only format.
		{
			Name:      "R2.1_custom_format_time_only",
			Args:      []string{"%H:%M:%S"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.2: custom format with full datetime.
		{
			Name:      "R2.2_custom_format_full_datetime",
			Args:      []string{"%a %b %e %T %Z %Y"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C", "TZ=UTC"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.3: subsecond extension %.S (seconds with microsecond suffix).
		{
			Name:      "R2.3_subsecond_dotS",
			Args:      []string{"%.S"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{secSubsecNormalizer},
		},
		// R2.3: subsecond extension %.s (Unix epoch with microsecond suffix).
		{
			Name:      "R2.3_subsecond_dots",
			Args:      []string{"%.s"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{epochSubsecNormalizer},
		},
		// R2.3: subsecond extension %.T (HH:MM:SS with microsecond suffix).
		{
			Name:      "R2.3_subsecond_dotT",
			Args:      []string{"%.T"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.3: subsecond extension %.T with multi-line input.
		{
			Name:      "R2.3_subsecond_dotT_multi_line",
			Args:      []string{"%.T"},
			Stdin:     []byte("line one\nline two\nline three\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.3: subsecond extension %.S with -s mode.
		{
			Name:      "R2.3_subsecond_dotS_elapsed",
			Args:      []string{"-s", "%.S"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{secSubsecNormalizer},
		},
		// R2.3: subsecond extension %.s with -i mode. Uses deltaSubsecNormalizer
		// because -i mode produces small delta epoch values (e.g., "0.000005").
		{
			Name:      "R2.3_subsecond_dots_incremental",
			Args:      []string{"-i", "%.s"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{deltaSubsecNormalizer},
		},
		// R2.3: mixed format with subsecond extensions.
		{
			Name:      "R2.3_mixed_format_with_subsecond",
			Args:      []string{"%Y-%m-%d %.T"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestR3_4_MutualExclusion verifies that passing both -i and -s prints a usage
// error to stderr and exits non-zero. R3.4: -i and -s are mutually exclusive.
// This cannot be a differential test because the reference ts binary accepts
// both flags without error; the PRD mandates stricter behavior.
func TestR3_4_MutualExclusion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "-i", "-s")
	cmd.Stdin = bytes.NewReader([]byte("test\n"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit code when -i and -s are both given")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() == 0 {
		t.Fatal("expected non-zero exit code when -i and -s are both given")
	}

	if !bytes.Contains(stderr.Bytes(), []byte("mutually exclusive")) {
		t.Errorf("expected stderr to mention 'mutually exclusive', got: %q", stderr.String())
	}
}
