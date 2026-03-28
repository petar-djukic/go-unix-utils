// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/true against GNU gtrue.
// Covers prd013-true R3.1-R3.2 (exit codes), R4.1-R4.3 (differential testing).
package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// helpVersionNormalizer normalizes --help and --version output so differences
// in binary paths, package names, version strings, ANSI escapes, and GNU
// trailer lines do not cause false failures. Exit code is still compared exactly.
func helpVersionNormalizer() testutils.NormalizeFunc {
	// Strip ANSI escape sequences (hyperlinks and bold used by GNU --help).
	ansiEsc := regexp.MustCompile(`\x1b(?:\][^\x1b]*\x1b\\|\[[0-9;]*m)`)
	// Normalize full binary paths to just "true" in Usage lines.
	binPath := regexp.MustCompile(`(?m)/[^\s]+/g?true`)
	// Matches "true (GNU coreutils) X.Y.Z" or "true (go-unix-utils) dev".
	versionLine := regexp.MustCompile(`(?m)^true \([^)]+\) .+$`)
	// Matches GNU trailer lines (copyright, license, bug reports, URLs, etc.).
	gnuTrailer := regexp.MustCompile(`(?m)^(Copyright|License|Written by|This is free|There is NO|Report |General help|or available|Full documentation|GNU coreutils).*\n?`)
	// Normalize "NOTE: your shell" vs "Your shell" phrasing.
	notePrefix := regexp.MustCompile(`(?m)^NOTE: (your shell)`)
	// Normalize option formatting: GNU puts description on next indented line;
	// our implementation puts it on the same line with padding.
	optSplit := regexp.MustCompile(`(--(?:help|version))\n\s+`)
	// Collapse runs of spaces to a single space in option lines.
	multiSpace := regexp.MustCompile(`(--(?:help|version))\s{2,}`)
	return func(b []byte) []byte {
		b = ansiEsc.ReplaceAll(b, nil)
		b = binPath.ReplaceAll(b, []byte("true"))
		b = versionLine.ReplaceAll(b, []byte("true (NORMALIZED) VERSION"))
		b = gnuTrailer.ReplaceAll(b, nil)
		b = notePrefix.ReplaceAll(b, []byte("Your shell"))
		b = optSplit.ReplaceAll(b, []byte("$1  "))
		b = multiSpace.ReplaceAll(b, []byte("$1 "))
		b = bytes.TrimRight(b, "\n")
		if len(b) > 0 {
			b = append(b, '\n')
		}
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtrue")
	if err != nil {
		t.Skipf("reference binary gtrue not in PATH: %v", err)
	}

	norm := helpVersionNormalizer()

	tests := []testutils.DiffTest{
		// R4.1, R4.3: no arguments — exit 0, no output.
		{
			Name: "no_args",
		},
		// R4.2: arbitrary arguments — exit 0, no output.
		{
			Name: "single_arg_ignored",
			Args: []string{"foo"},
		},
		{
			Name: "multiple_args_ignored",
			Args: []string{"foo", "bar", "--baz"},
		},
		// R4.2: single dash argument — exit 0, no output.
		{
			Name: "dash_arg",
			Args: []string{"-"},
		},
		// R4.2: flags that look like options — still exit 0, no output.
		{
			Name: "unknown_flag_ignored",
			Args: []string{"--unknown-flag"},
		},
		// R4.2: --help output.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		// R4.2: --version output.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		// R3.1: --help not first arg — treated as ignored arg, exit 0.
		{
			Name: "help_not_first",
			Args: []string{"foo", "--help"},
		},
		// R3.1: --version not first arg — treated as ignored arg, exit 0.
		{
			Name: "version_not_first",
			Args: []string{"foo", "--version"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
