// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sleep against GNU gsleep.
// Covers prd061-sleep R4.1 (valid duration exit codes), R4.2 (invalid argument errors),
// R4.3 (--version/--help flags), R4.4 (multiple args, zero, edge cases).
package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// binPathRe matches full binary paths like /opt/homebrew/bin/gsleep or /opt/homebrew/bin/sleep.
var binPathRe = regexp.MustCompile(`/[^\s']+/g?sleep`)

// progNameRe matches the gsleep program name (not preceded by /).
var progNameRe = regexp.MustCompile(`\bgsleep\b`)

// stderrNormalizer replaces full binary paths and gsleep references with
// "sleep" so program-name and path differences do not cause false failures.
func stderrNormalizer() testutils.NormalizeFunc {
	return func(b []byte) []byte {
		b = binPathRe.ReplaceAll(b, []byte("sleep"))
		b = progNameRe.ReplaceAll(b, []byte("sleep"))
		return b
	}
}

// exitCodeOnlyNormalizer clears all output so only exit codes are compared.
// Used when error message formats differ fundamentally between GNU and Go.
func exitCodeOnlyNormalizer() testutils.NormalizeFunc {
	return func(b []byte) []byte {
		return nil
	}
}

// helpVersionNormalizer normalizes --help and --version output so differences
// in binary paths, package names, version strings, ANSI escapes, and GNU
// trailer lines do not cause false failures.
func helpVersionNormalizer() testutils.NormalizeFunc {
	ansiEsc := regexp.MustCompile(`\x1b(?:\][^\x1b]*\x1b\\|\[[0-9;]*m)`)
	versionLine := regexp.MustCompile(`(?m)^sleep \([^)]+\).*$`)
	gnuTrailer := regexp.MustCompile(`(?m)^(Copyright|License|Written by|This is free|There is NO|Report |General help|or available|Full documentation|GNU coreutils).*\n?`)
	stderrNorm := stderrNormalizer()
	return func(b []byte) []byte {
		b = ansiEsc.ReplaceAll(b, nil)
		b = stderrNorm(b)
		b = versionLine.ReplaceAll(b, []byte("sleep (NORMALIZED) VERSION"))
		b = gnuTrailer.ReplaceAll(b, nil)
		b = bytes.TrimRight(b, "\n")
		if len(b) > 0 {
			b = append(b, '\n')
		}
		return b
	}
}

// helpBodyNormalizer strips the help body text, keeping only the usage line
// prefix and exit code. Help body prose differs between GNU and Go.
func helpBodyNormalizer() testutils.NormalizeFunc {
	usageLine := regexp.MustCompile(`(?m)^Usage: sleep `)
	hvNorm := helpVersionNormalizer()
	return func(b []byte) []byte {
		b = hvNorm(b)
		// Keep only whether "Usage: sleep" appears; strip details.
		if usageLine.Match(b) {
			return []byte("Usage: sleep PRESENT\n")
		}
		return b
	}
}

// TestDiff runs differential tests for sleep against gsleep.
// R4.1: valid durations exit 0. R4.2: invalid args exit 1.
// R4.3: --version/--help flags. R4.4: multiple args, zero, edge cases.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsleep")
	if err != nil {
		t.Skipf("reference binary gsleep not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()
	exitOnly := exitCodeOnlyNormalizer()
	hvNorm := helpBodyNormalizer()
	verNorm := helpVersionNormalizer()

	tests := []testutils.DiffTest{
		// R4.1: zero duration exits immediately with code 0.
		{
			Name: "zero_duration",
			Args: []string{"0"},
		},
		// R4.1: very small fractional seconds.
		{
			Name: "fractional_seconds",
			Args: []string{"0.001"},
		},
		// R4.4: zero with 's' suffix.
		{
			Name: "zero_s_suffix",
			Args: []string{"0s"},
		},
		// R4.4: zero with 'm' suffix.
		{
			Name: "zero_m_suffix",
			Args: []string{"0m"},
		},
		// R4.4: zero with 'h' suffix.
		{
			Name: "zero_h_suffix",
			Args: []string{"0h"},
		},
		// R4.4: zero with 'd' suffix.
		{
			Name: "zero_d_suffix",
			Args: []string{"0d"},
		},
		// R4.4: multiple arguments summed (all zero → immediate exit 0).
		{
			Name: "multiple_args_summed",
			Args: []string{"0", "0", "0"},
		},
		// R4.4: multiple arguments with mixed suffixes.
		{
			Name: "multiple_args_mixed_suffix",
			Args: []string{"0s", "0m", "0h"},
		},
		// R4.4: very small fractional with suffix.
		{
			Name: "small_fractional_with_suffix",
			Args: []string{"0.001s"},
		},
		// R4.2: no arguments → error, exit 1.
		{
			Name:      "no_args_error",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.2: non-numeric argument → error, exit 1.
		{
			Name:      "invalid_arg_abc",
			Args:      []string{"abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.2: negative argument → error, exit 1.
		// GNU treats -1 as an invalid option; Go treats it as invalid interval.
		// Compare only exit codes since error message formats differ.
		{
			Name:      "negative_arg",
			Args:      []string{"-1"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{exitOnly},
		},
		// R4.2: empty string argument → error, exit 1.
		{
			Name:      "empty_string_arg",
			Args:      []string{""},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3: --help flag — compare structure, not prose.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{hvNorm},
		},
		// R4.3: --version flag.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{verNorm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
