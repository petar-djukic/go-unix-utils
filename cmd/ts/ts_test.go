// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements differential tests for cmd/ts against the reference ts binary.
// Implements: prd004-ts R9, prd001-testutils R1-R4
// Test suite: test-rel01.0
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var (
	goBinaryPath string
	refBinaryPath string
)

// TestMain builds the cmd/ts binary and locates the reference ts binary before
// running any tests. All tests are skipped when the reference binary is not found.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ts-test-*")
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}

	goBinaryPath = filepath.Join(dir, "ts")
	build := exec.Command("go", "build", "-o", goBinaryPath, ".")
	build.Stdout = os.Stderr
	build.Stderr = os.Stderr
	if buildErr := build.Run(); buildErr != nil {
		os.RemoveAll(dir)
		panic("failed to build ts binary: " + buildErr.Error())
	}

	if refPath, lookErr := exec.LookPath("ts"); lookErr == nil {
		refBinaryPath = refPath
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// TestTs runs all 13 differential test cases from test-rel01.0 against the
// reference ts binary (from moreutils). Each test case exercises a requirement
// group from prd004-ts R9.
func TestTs(t *testing.T) {
	if refBinaryPath == "" {
		t.Skip("reference binary 'ts' not found on PATH; install moreutils to run differential tests")
	}

	tests := []testutils.DiffTest{
		{
			// prd004-ts R1.1, R1.2, R1.4 — default format with three-line stdin
			Name:             "default_format_three_lines",
			Args:             nil,
			Stdin:            []byte("line1\nline2\nline3\n"),
			Env:              []string{"LC_ALL=C"},
			ExpectedExitCode: 0,
			Normalize:        []testutils.Normalize{testutils.TimestampNormalizer},
		},
		{
			// prd004-ts R1.6 — empty stdin produces no output, exits 0
			Name:             "default_format_empty_stdin",
			Args:             nil,
			Stdin:            []byte{},
			Env:              []string{"LC_ALL=C"},
			ExpectedExitCode: 0,
			Normalize:        nil,
		},
		{
			// prd004-ts R1.5 — partial last line (no trailing newline) must be timestamped
			Name:             "default_format_partial_last_line",
			Args:             nil,
			Stdin:            []byte("partial"),
			Env:              []string{"LC_ALL=C"},
			ExpectedExitCode: 0,
			Normalize:        []testutils.Normalize{testutils.TimestampNormalizer},
		},
		{
			// prd004-ts R2.1, R2.2 — custom strftime format string
			Name:             "custom_strftime_format",
			Args:             []string{"%Y-%m-%dT%H:%M:%S"},
			Stdin:            []byte("hello\nworld\n"),
			Env:              []string{"LC_ALL=C"},
			ExpectedExitCode: 0,
			Normalize:        []testutils.Normalize{testutils.TimestampNormalizer},
		},
		{
			// prd004-ts R2.3, R2.4 — subsecond extension %.S
			Name:             "subsecond_format_dotS",
			Args:             []string{"%.S"},
			Stdin:            []byte("test\n"),
			Env:              []string{"LC_ALL=C"},
			ExpectedExitCode: 0,
			Normalize:        []testutils.Normalize{testutils.TimestampNormalizer},
		},
		{
			// prd004-ts R4.1, R4.2 — elapsed-since-start mode; TZ=GMT for correct elapsed formatting
			Name:             "elapsed_since_start_mode",
			Args:             []string{"-s"},
			Stdin:            []byte("a\nb\nc\n"),
			Env:              []string{"LC_ALL=C", "TZ=GMT"},
			ExpectedExitCode: 0,
			Normalize:        []testutils.Normalize{testutils.TimestampNormalizer},
		},
		{
			// prd004-ts R3.1, R3.2 — incremental mode; TZ=GMT for correct elapsed formatting
			Name:             "incremental_mode",
			Args:             []string{"-i"},
			Stdin:            []byte("first\nsecond\nthird\n"),
			Env:              []string{"LC_ALL=C", "TZ=GMT"},
			ExpectedExitCode: 0,
			Normalize:        []testutils.Normalize{testutils.TimestampNormalizer},
		},
		{
			// prd004-ts R3.4 — -i and -s are mutually exclusive; must exit non-zero
			Name:             "incremental_and_elapsed_mutually_exclusive",
			Args:             []string{"-i", "-s"},
			Stdin:            nil,
			Env:              []string{"LC_ALL=C"},
			ExpectedExitCode: 1,
			Normalize:        nil,
		},
		{
			// prd004-ts R5.1, R5.2 — monotonic clock mode
			Name:             "monotonic_clock_mode",
			Args:             []string{"-m"},
			Stdin:            []byte("tick\n"),
			Env:              []string{"LC_ALL=C"},
			ExpectedExitCode: 0,
			Normalize:        []testutils.Normalize{testutils.TimestampNormalizer},
		},
		{
			// prd004-ts R6.1, R6.2 — relative mode converts syslog-format timestamp
			Name:             "relative_mode_syslog_timestamp",
			Args:             []string{"-r"},
			Stdin:            []byte("Jan  1 00:00:00 boot message\n"),
			Env:              []string{"LC_ALL=C"},
			ExpectedExitCode: 0,
			Normalize:        []testutils.Normalize{testutils.TimestampNormalizer},
		},
		{
			// prd004-ts R6.4 — relative mode passes through lines with no timestamp unchanged
			Name:             "relative_mode_no_timestamp_passthrough",
			Args:             []string{"-r"},
			Stdin:            []byte("no timestamp here\n"),
			Env:              []string{"LC_ALL=C"},
			ExpectedExitCode: 0,
			Normalize:        nil,
		},
		{
			// prd004-ts R7.2 — unrecognized flag must exit non-zero
			Name:             "invalid_flag",
			Args:             []string{"-z"},
			Stdin:            nil,
			Env:              []string{"LC_ALL=C"},
			ExpectedExitCode: 1,
			Normalize:        nil,
		},
		{
			// prd004-ts R8.1 — TZ=UTC causes wall-clock timestamps to be in UTC
			Name:             "tz_environment_respected",
			Args:             []string{"%H:%M:%S"},
			Stdin:            []byte("event\n"),
			Env:              []string{"LC_ALL=C", "TZ=UTC"},
			ExpectedExitCode: 0,
			Normalize:        []testutils.Normalize{testutils.TimestampNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}
