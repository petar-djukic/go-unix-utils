// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ts exercising all test cases from
// test-rel01.0.yaml.
//
// Implements: prd004-ts R9 (differential testing), prd001-testutils R1-R3
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goBinary is the path to the freshly built Go ts binary. Set by TestMain.
var goBinary string

// refBinary is the path to the Homebrew reference ts binary. Set by TestMain.
var refBinary string

// baseEnv provides the standard test environment per test-rel01.0.yaml
// preconditions: LC_ALL=C and a fixed timezone for consistent strftime output.
var baseEnv = []string{"LC_ALL=C", "TZ=America/New_York"}

// relativeAgePattern matches relative age strings produced by -r mode,
// covering both Go format (e.g., "55d15h ago") and Perl reference format
// (e.g., "55d15h2m30s ago").
var relativeAgePattern = regexp.MustCompile(`\d+[ydhms](?:\d+[ydhms])* ago`)

// relativeAgeNormalizer replaces relative age strings with a fixed placeholder
// to eliminate non-deterministic age output in -r mode differential tests.
func relativeAgeNormalizer(b []byte) []byte {
	return relativeAgePattern.ReplaceAll(b, []byte("TIMESTAMP"))
}

// TestMain builds the Go ts binary and locates the Homebrew reference binary.
// Per design decision D4.
func TestMain(m *testing.M) {
	// Build the Go ts binary into a temp directory.
	tmpDir, err := os.MkdirTemp("", "ts-test-*")
	if err != nil {
		os.Exit(1)
	}

	goBinary = filepath.Join(tmpDir, "ts")
	buildCmd := exec.Command("go", "build", "-o", goBinary, ".")
	if _, err := buildCmd.CombinedOutput(); err != nil {
		// Build failed; leave goBinary empty so tests skip gracefully.
		goBinary = ""
	}

	// Locate the Homebrew reference binary (brew install moreutils).
	refBinary, _ = exec.LookPath("ts")

	code := m.Run()
	// Best-effort cleanup of temp directory.
	_ = os.RemoveAll(tmpDir)
	os.Exit(code)
}

// TestTsDifferential runs all differential test cases from test-rel01.0.yaml.
// Per prd001-testutils AC1, the test defines a []DiffTest slice and calls
// RunDiffTests(t, goBinary, refBinary, tests).
func TestTsDifferential(t *testing.T) {
	if goBinary == "" {
		t.Skip("Go ts binary could not be built; skipping differential tests")
	}
	if refBinary == "" {
		t.Skip("reference ts binary not found on PATH (brew install moreutils); skipping differential tests")
	}

	tests := []testutils.DiffTest{
		// --- Wall-clock timestamp tests (prd004-ts R1, R2) ---

		{
			// Per test-rel01.0.yaml: default_format_three_lines.
			// Traces: prd004-ts R1.1, R1.2, R1.4.
			Name:      "default_format_three_lines",
			Args:      nil,
			Stdin:     []byte("line1\nline2\nline3\n"),
			Env:       baseEnv,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// Per test-rel01.0.yaml: default_format_empty_stdin.
			// Traces: prd004-ts R1.6.
			Name:      "default_format_empty_stdin",
			Args:      nil,
			Stdin:     []byte{},
			Env:       baseEnv,
			Normalize: nil,
		},
		{
			// Per test-rel01.0.yaml: default_format_partial_last_line.
			// Traces: prd004-ts R1.5.
			Name:      "default_format_partial_last_line",
			Args:      nil,
			Stdin:     []byte("partial"),
			Env:       baseEnv,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// Per test-rel01.0.yaml: custom_strftime_format.
			// Traces: prd004-ts R2.1, R2.2.
			Name:      "custom_strftime_format",
			Args:      []string{"%Y-%m-%dT%H:%M:%S"},
			Stdin:     []byte("hello\nworld\n"),
			Env:       baseEnv,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// Per test-rel01.0.yaml: subsecond_format_dotS.
			// Traces: prd004-ts R2.3, R2.4.
			Name:      "subsecond_format_dotS",
			Args:      []string{"%.S"},
			Stdin:     []byte("test\n"),
			Env:       baseEnv,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},

		// --- Elapsed-mode tests (prd004-ts R3, R4) ---

		{
			// Per test-rel01.0.yaml: elapsed_since_start_mode.
			// Traces: prd004-ts R4.1, R4.2.
			Name:      "elapsed_since_start_mode",
			Args:      []string{"-s"},
			Stdin:     []byte("a\nb\nc\n"),
			Env:       baseEnv,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			// Per test-rel01.0.yaml: incremental_mode.
			// Traces: prd004-ts R3.1, R3.2.
			Name:      "incremental_mode",
			Args:      []string{"-i"},
			Stdin:     []byte("first\nsecond\nthird\n"),
			Env:       baseEnv,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},

		// --- Error and mutual-exclusivity tests (prd004-ts R3.4, R7.2) ---

		{
			// Per test-rel01.0.yaml: incremental_and_elapsed_mutually_exclusive.
			// Traces: prd004-ts R3.4.
			Name:      "incremental_and_elapsed_mutually_exclusive",
			Args:      []string{"-i", "-s"},
			Stdin:     []byte{},
			Env:       baseEnv,
			Normalize: nil,
		},
		{
			// Per test-rel01.0.yaml: invalid_flag.
			// Traces: prd004-ts R7.2.
			Name:      "invalid_flag",
			Args:      []string{"-z"},
			Stdin:     []byte{},
			Env:       baseEnv,
			Normalize: nil,
		},

		// --- Monotonic clock test (prd004-ts R5) ---

		{
			// Per test-rel01.0.yaml: monotonic_clock_mode.
			// Traces: prd004-ts R5.1, R5.2.
			Name:      "monotonic_clock_mode",
			Args:      []string{"-m"},
			Stdin:     []byte("tick\n"),
			Env:       baseEnv,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},

		// --- Relative-mode tests (prd004-ts R6) ---

		{
			// Per test-rel01.0.yaml: relative_mode_syslog_timestamp.
			// Traces: prd004-ts R6.1, R6.2.
			Name:      "relative_mode_syslog_timestamp",
			Args:      []string{"-r"},
			Stdin:     []byte("Jan  1 00:00:00 boot message\n"),
			Env:       baseEnv,
			Normalize: []testutils.NormalizeFunc{relativeAgeNormalizer, testutils.TimestampNormalizer},
		},
		{
			// Per test-rel01.0.yaml: relative_mode_no_timestamp_passthrough.
			// Traces: prd004-ts R6.4.
			Name:      "relative_mode_no_timestamp_passthrough",
			Args:      []string{"-r"},
			Stdin:     []byte("no timestamp here\n"),
			Env:       baseEnv,
			Normalize: nil,
		},

		// --- Environment test (prd004-ts R8) ---

		{
			// Per test-rel01.0.yaml: tz_environment_respected.
			// Traces: prd004-ts R8.1.
			Name:      "tz_environment_respected",
			Args:      []string{"%H:%M:%S"},
			Stdin:     []byte("event\n"),
			Env:       []string{"LC_ALL=C", "TZ=UTC"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBinary, refBinary, tests)
}
