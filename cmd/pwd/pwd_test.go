// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/pwd against GNU gpwd.
// Covers prd051-pwd R3.1-R3.3 (differential testing, coverage, LC_ALL=C).
package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// helpVersionNormalizer normalizes --help and --version output so differences
// in binary paths, package names, version strings, GNU trailer lines, and
// shell-specific paragraphs do not cause false failures.
func helpVersionNormalizer() testutils.NormalizeFunc {
	ansiEsc := regexp.MustCompile(`\x1b(?:\][^\x1b]*\x1b\\|\[[0-9;]*m)`)
	binPath := regexp.MustCompile(`(?m)/[^\s]+/g?pwd`)
	versionLine := regexp.MustCompile(`(?m)^pwd \([^)]+\) .+$`)
	gnuTrailer := regexp.MustCompile(`(?m)^(Copyright|License|Written by|This is free|There is NO|Report |General help|or available|Full documentation|GNU coreutils|Your shell).*\n?`)
	physDesc := regexp.MustCompile(`--physical\s+(avoid all symlinks|resolve all symlinks)`)
	optWrap := regexp.MustCompile(`(--(?:help|version|logical|physical))\n\s+`)
	optSpace := regexp.MustCompile(`(--(?:help|version|logical|physical))\s{2,}`)
	blankLine := regexp.MustCompile(`\n\n(\s+--|-[LP])`)
	detailLine := regexp.MustCompile(`(?m)^.*Please refer to.*\n?`)
	forDetails := regexp.MustCompile(`(?m)^.*for details about.*\n?`)
	return func(b []byte) []byte {
		b = ansiEsc.ReplaceAll(b, nil)
		b = binPath.ReplaceAll(b, []byte("pwd"))
		b = versionLine.ReplaceAll(b, []byte("pwd (NORMALIZED) VERSION"))
		b = gnuTrailer.ReplaceAll(b, nil)
		b = detailLine.ReplaceAll(b, nil)
		b = forDetails.ReplaceAll(b, nil)
		b = physDesc.ReplaceAll(b, []byte("--physical NORMALIZED_DESC"))
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
	binPath := regexp.MustCompile(`/[^\s:]+/g?pwd|gpwd`)
	tryHelp := regexp.MustCompile(`(?m)^Try '[^']*' for more information\.\n?`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("pwd"))
		b = tryHelp.ReplaceAll(b, nil)
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpwd")
	if err != nil {
		t.Skipf("reference binary gpwd not in PATH: %v", err)
	}

	norm := helpVersionNormalizer()
	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		// R3.2: default invocation — no arguments, prints physical cwd.
		{
			Name: "no_args",
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: -P flag — physical path (explicit).
		{
			Name: "physical_flag",
			Args: []string{"-P"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: -L flag — logical path from PWD.
		{
			Name: "logical_flag",
			Args: []string{"-L"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: -L -P precedence — last flag wins, physical.
		{
			Name: "logical_then_physical",
			Args: []string{"-L", "-P"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: -P -L precedence — last flag wins, logical.
		{
			Name: "physical_then_logical",
			Args: []string{"-P", "-L"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: GNU pwd ignores non-option arguments, prints cwd, exits 0.
		{
			Name:      "extra_operand",
			Args:      []string{"extraarg"},
			ExitCode:  0,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R2.2: unknown flag error — exit 1.
		{
			Name:      "unknown_flag",
			Args:      []string{"--invalid"},
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
