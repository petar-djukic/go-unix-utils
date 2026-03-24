// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/nproc against GNU gnproc.
// Covers prd046-nproc R3.1-R3.3 (differential testing, test coverage, LC_ALL=C).
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
	binPath := regexp.MustCompile(`(?m)/[^\s]+/g?nproc`)
	versionLine := regexp.MustCompile(`(?m)^nproc \([^)]+\) .+$`)
	gnuTrailer := regexp.MustCompile(`(?m)^(Copyright|License|Written by|This is free|There is NO|Report |General help|or available|Full documentation|GNU coreutils).*\n?`)
	ompPara := regexp.MustCompile(`(?m)^If the 'OMP_NUM_THREADS'.*\n(?:.*\S.*\n)*`)
	// Normalize option lines: keep only "      --option\n", strip
	// description text and continuation lines.
	optLine := regexp.MustCompile(`(?m)^( +--\S+)[^\n]*\n(?:[ \t]+[^\-\n][^\n]*\n)*`)
	return func(b []byte) []byte {
		b = ansiEsc.ReplaceAll(b, nil)
		b = binPath.ReplaceAll(b, []byte("nproc"))
		b = versionLine.ReplaceAll(b, []byte("nproc (NORMALIZED) VERSION"))
		b = gnuTrailer.ReplaceAll(b, nil)
		b = ompPara.ReplaceAll(b, nil)
		b = optLine.ReplaceAll(b, []byte("$1\n"))
		b = bytes.TrimRight(b, "\n")
		if len(b) > 0 {
			b = append(b, '\n')
		}
		return b
	}
}

// stderrNormalizer normalizes error messages between GNU and Go binaries.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?nproc|gnproc`)
	tryHelp := regexp.MustCompile(`(?m)^Try '[^']*' for more information\.\n?`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("nproc"))
		b = tryHelp.ReplaceAll(b, nil)
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnproc")
	if err != nil {
		t.Skipf("reference binary gnproc not in PATH: %v", err)
	}

	norm := helpVersionNormalizer()
	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		// R3.2: default invocation — no arguments, prints available CPU count.
		{
			Name: "no_args",
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: --all prints installed processor count.
		{
			Name: "all_flag",
			Args: []string{"--all"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: --ignore=1 subtracts one from available count.
		{
			Name: "ignore_one",
			Args: []string{"--ignore=1"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: --all --ignore=1 combined.
		{
			Name: "all_ignore_one",
			Args: []string{"--all", "--ignore=1"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: --ignore with large N clamps result to 1.
		{
			Name: "ignore_large_clamp",
			Args: []string{"--ignore=99999"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: unknown flag — exit 1.
		{
			Name:      "unknown_flag",
			Args:      []string{"--invalid"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R3.2: unknown short flag — exit 1.
		{
			Name:      "unknown_short_flag",
			Args:      []string{"-x"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R3.2: extra operand — exit 1.
		{
			Name:      "extra_operand",
			Args:      []string{"foo"},
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
