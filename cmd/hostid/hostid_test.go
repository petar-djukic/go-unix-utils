// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/hostid against GNU ghostid.
// Covers prd048-hostid R3.1-R3.2 (differential testing, test coverage).
package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// helpVersionNormalizer normalizes --help and --version output so differences
// in binary paths, package names, version strings, and GNU trailer lines
// do not cause false failures.
func helpVersionNormalizer() testutils.NormalizeFunc {
	ansiEsc := regexp.MustCompile(`\x1b(?:\][^\x1b]*\x1b\\|\[[0-9;]*m)`)
	binPath := regexp.MustCompile(`(?m)/[^\s]+/g?hostid`)
	versionLine := regexp.MustCompile(`(?m)^hostid \([^)]+\) .+$`)
	gnuTrailer := regexp.MustCompile(`(?m)^(Copyright|License|Written by|This is free|There is NO|Report |General help|or available|Full documentation|GNU coreutils).*\n?`)
	optWrap := regexp.MustCompile(`(--(?:help|version))\n\s+`)
	optSpace := regexp.MustCompile(`(--(?:help|version))\s{2,}`)
	blankLine := regexp.MustCompile(`\n\n(\s+--(?:help|version))`)
	return func(b []byte) []byte {
		b = ansiEsc.ReplaceAll(b, nil)
		b = binPath.ReplaceAll(b, []byte("hostid"))
		b = versionLine.ReplaceAll(b, []byte("hostid (NORMALIZED) VERSION"))
		b = gnuTrailer.ReplaceAll(b, nil)
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
	binPath := regexp.MustCompile(`/[^\s:]+/g?hostid|ghostid`)
	tryHelp := regexp.MustCompile(`(?m)^Try '[^']*' for more information\.\n?`)
	// GNU uses "invalid option -- 'x'"; Go uses "unrecognized option '-x'".
	shortOpt := regexp.MustCompile(`(?:invalid option -- '([^']+)'|unrecognized option '-([^']+)')`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("hostid"))
		b = tryHelp.ReplaceAll(b, nil)
		b = shortOpt.ReplaceAll(b, []byte("invalid option $1$2"))
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ghostid")
	if err != nil {
		t.Skipf("reference binary ghostid not in PATH: %v", err)
	}

	norm := helpVersionNormalizer()
	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		// R3.2: normal invocation — no arguments, prints host identifier.
		{
			Name: "no_args",
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: extra operand error — exit 1.
		{
			Name:      "extra_operand",
			Args:      []string{"extraarg"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R3.2: unknown flag error — exit 1.
		{
			Name:      "unknown_flag",
			Args:      []string{"--invalid"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R3.2: unknown short flag error — exit 1.
		{
			Name:      "unknown_short_flag",
			Args:      []string{"-x"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// --help output.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		// --version output.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
