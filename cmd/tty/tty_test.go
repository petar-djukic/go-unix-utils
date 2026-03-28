// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tty against GNU gtty.
// Covers prd052-tty R3.1-R3.3 (differential testing, coverage, LC_ALL=C).
package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// helpVersionNormalizer normalizes --help and --version output differences
// between GNU and Go binaries.
func helpVersionNormalizer() testutils.NormalizeFunc {
	ansiEsc := regexp.MustCompile(`\x1b(?:\][^\x1b]*\x1b\\|\[[0-9;]*m)`)
	binPath := regexp.MustCompile(`(?m)/[^\s]+/g?tty`)
	versionLine := regexp.MustCompile(`(?m)^tty \([^)]+\) .+$`)
	gnuTrailer := regexp.MustCompile(`(?m)^(Copyright|License|Written by|This is free|There is NO|Report |General help|or available|Full documentation|GNU coreutils|Your shell).*\n?`)
	optWrap := regexp.MustCompile(`(--(?:help|version|silent|quiet))\n\s+`)
	optSpace := regexp.MustCompile(`(--(?:help|version|silent|quiet))\s{2,}`)
	blankLine := regexp.MustCompile(`\n\n(\s+--|-)`)
	detailLine := regexp.MustCompile(`(?m)^.*Please refer to.*\n?`)
	forDetails := regexp.MustCompile(`(?m)^.*for details about.*\n?`)
	return func(b []byte) []byte {
		b = ansiEsc.ReplaceAll(b, nil)
		b = binPath.ReplaceAll(b, []byte("tty"))
		b = versionLine.ReplaceAll(b, []byte("tty (NORMALIZED) VERSION"))
		b = gnuTrailer.ReplaceAll(b, nil)
		b = detailLine.ReplaceAll(b, nil)
		b = forDetails.ReplaceAll(b, nil)
		b = optWrap.ReplaceAll(b, []byte("$1 "))
		b = optSpace.ReplaceAll(b, []byte("$1 "))
		b = blankLine.ReplaceAll(b, []byte("\n$1"))
		b = bytes.TrimRight(b, "\n")
		if len(b) > 0 {
			b = append(b, '\n')
		}
		return b
	}
}

// stderrNormalizer normalizes error messages between GNU and Go binaries.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?tty|gtty`)
	tryHelp := regexp.MustCompile(`(?m)^Try '[^']*' for more information\.\n?`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("tty"))
		b = tryHelp.ReplaceAll(b, nil)
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtty")
	if err != nil {
		t.Skipf("reference binary gtty not in PATH: %v", err)
	}

	norm := helpVersionNormalizer()
	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		// R1.2, R3.2: stdin not a tty (piped) — prints "not a tty", exit 1.
		{
			Name:     "pipe_stdin_not_tty",
			ExitCode: 1,
			Env:      []string{"LC_ALL=C"},
		},
		// R1.3, R3.2: -s flag with pipe stdin — no output, exit 1.
		{
			Name:     "silent_short_flag",
			Args:     []string{"-s"},
			ExitCode: 1,
			Env:      []string{"LC_ALL=C"},
		},
		// R1.3: --silent long form with pipe stdin.
		{
			Name:     "silent_long_flag",
			Args:     []string{"--silent"},
			ExitCode: 1,
			Env:      []string{"LC_ALL=C"},
		},
		// R1.3: --quiet long form with pipe stdin.
		{
			Name:     "quiet_long_flag",
			Args:     []string{"--quiet"},
			ExitCode: 1,
			Env:      []string{"LC_ALL=C"},
		},
		// R2.1: extra operand — error to stderr, exit 2.
		{
			Name:      "extra_operand",
			Args:      []string{"extra"},
			ExitCode:  2,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R2.2: unknown short flag — error to stderr, exit 2.
		{
			Name:      "unknown_short_flag",
			Args:      []string{"-x"},
			ExitCode:  2,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R2.2: unknown long flag — error to stderr, exit 2.
		{
			Name:      "unknown_long_flag",
			Args:      []string{"--invalid"},
			ExitCode:  2,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R3.2: --help output.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		// R3.2: --version output.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
