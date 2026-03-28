// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/logname against GNU glogname.
// Covers prd053-logname R3.2-R3.3 (differential testing, LC_ALL=C).
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
	binPath := regexp.MustCompile(`(?m)/[^\s]+/g?logname`)
	versionLine := regexp.MustCompile(`(?m)^logname \([^)]+\) .+$`)
	gnuTrailer := regexp.MustCompile(`(?m)^(Copyright|License|Written by|This is free|There is NO|Report |General help|or available|Full documentation|GNU coreutils|Your shell).*\n?`)
	optWrap := regexp.MustCompile(`(--(?:help|version))\n\s+`)
	optSpace := regexp.MustCompile(`(--(?:help|version))\s{2,}`)
	blankLine := regexp.MustCompile(`\n\n(\s+--|-)`)
	descLine := regexp.MustCompile(`(?m)^Print .+(login name|current user)\.$`)
	return func(b []byte) []byte {
		b = ansiEsc.ReplaceAll(b, nil)
		b = binPath.ReplaceAll(b, []byte("logname"))
		b = versionLine.ReplaceAll(b, []byte("logname (NORMALIZED) VERSION"))
		b = gnuTrailer.ReplaceAll(b, nil)
		b = descLine.ReplaceAll(b, []byte("Print NORMALIZED login name."))
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
	binPath := regexp.MustCompile(`/[^\s:]+/g?logname|glogname`)
	tryHelp := regexp.MustCompile(`(?m)^Try '[^']*' for more information\.\n?`)
	optErr := regexp.MustCompile(`(invalid option -- '.'|unrecognized option '[^']+')`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("logname"))
		b = tryHelp.ReplaceAll(b, nil)
		b = optErr.ReplaceAll(b, []byte("NORMALIZED_OPT_ERR"))
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("glogname")
	if err != nil {
		t.Skipf("reference binary glogname not in PATH: %v", err)
	}

	norm := helpVersionNormalizer()
	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		// R3.2: no arguments — prints login name, exit 0.
		{
			Name: "default_no_args",
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: extra operand — error to stderr, exit 1.
		{
			Name:      "extra_operand",
			Args:      []string{"extraarg"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R3.2: unknown short flag — error to stderr, exit 1.
		{
			Name:      "unknown_short_flag",
			Args:      []string{"-x"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R3.2: unknown long flag — error to stderr, exit 1.
		{
			Name:      "unknown_long_flag",
			Args:      []string{"--invalid"},
			ExitCode:  1,
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
