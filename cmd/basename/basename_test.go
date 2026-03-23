// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/basename against GNU gbasename.
// Covers prd015-basename R3.1-R3.4 (exit codes, error messages),
// R4.1-R4.3 (differential testing).
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
	binPath := regexp.MustCompile(`(?m)/[^\s]+/g?basename`)
	versionLine := regexp.MustCompile(`(?m)^basename \([^)]+\) .+$`)
	gnuTrailer := regexp.MustCompile(`(?m)^(Copyright|License|Written by|This is free|There is NO|Report |General help|or available|Full documentation|GNU coreutils).*\n?`)
	// Normalize option description that wraps to next indented line.
	optWrap := regexp.MustCompile(`(--(?:help|version|multiple|suffix=SUFFIX|zero))\n\s+`)
	// Collapse runs of whitespace after option flags to single space.
	optSpace := regexp.MustCompile(`(--(?:help|version|multiple|suffix=SUFFIX|zero))\s{2,}`)
	// Remove blank lines within option blocks.
	blankLine := regexp.MustCompile(`\n\n(\s+--(?:help|version))`)
	return func(b []byte) []byte {
		b = ansiEsc.ReplaceAll(b, nil)
		b = binPath.ReplaceAll(b, []byte("basename"))
		b = versionLine.ReplaceAll(b, []byte("basename (NORMALIZED) VERSION"))
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
	// Normalize full paths and gbasename to basename.
	binPath := regexp.MustCompile(`/[^\s:]+/g?basename|gbasename`)
	// Remove GNU "Try ... --help" hint lines (Go binary may omit these).
	tryHelp := regexp.MustCompile(`(?m)^Try '[^']*' for more information\.\n?`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("basename"))
		b = tryHelp.ReplaceAll(b, nil)
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbasename")
	if err != nil {
		t.Skipf("reference binary gbasename not in PATH: %v", err)
	}

	norm := helpVersionNormalizer()
	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		// R4.1/R4.2: simple path — strip directory.
		{
			Name: "simple_path",
			Args: []string{"/usr/bin/sort"},
		},
		// R4.2: filename only — no directory to strip.
		{
			Name: "no_directory",
			Args: []string{"sort"},
		},
		// R4.2: trailing slashes stripped before processing.
		{
			Name: "trailing_slashes",
			Args: []string{"/usr/bin/sort///"},
		},
		// R4.2: root path.
		{
			Name: "root_path",
			Args: []string{"/"},
		},
		// R4.2: multiple slashes only.
		{
			Name: "all_slashes",
			Args: []string{"///"},
		},
		// R4.2: empty string argument.
		{
			Name: "empty_string",
			Args: []string{""},
		},
		// R4.2: suffix removal (two-argument form).
		{
			Name: "suffix_removal",
			Args: []string{"include/stdio.h", ".h"},
		},
		// R4.2: suffix does not match — no removal.
		{
			Name: "suffix_no_match",
			Args: []string{"include/stdio.c", ".h"},
		},
		// R4.2: suffix equals entire basename — no removal.
		{
			Name: "suffix_equals_name",
			Args: []string{".h", ".h"},
		},
		// R4.2: -a multi-argument mode.
		{
			Name: "multi_arg_mode",
			Args: []string{"-a", "/usr/bin/sort", "/usr/bin/ls"},
		},
		// R4.2: -a with single argument.
		{
			Name: "multi_single",
			Args: []string{"-a", "/usr/bin/sort"},
		},
		// R4.2: --multiple long form.
		{
			Name: "multiple_long",
			Args: []string{"--multiple", "/usr/bin/sort", "/usr/bin/ls"},
		},
		// R4.2: -s suffix mode (implies -a).
		{
			Name: "suffix_flag",
			Args: []string{"-s", ".h", "include/stdio.h", "include/stdlib.h"},
		},
		// R4.2: --suffix= long form.
		{
			Name: "suffix_long",
			Args: []string{"--suffix=.h", "include/stdio.h", "include/stdlib.h"},
		},
		// R4.2: -z NUL-terminated output.
		{
			Name: "zero_flag",
			Args: []string{"-z", "foo"},
		},
		// R4.2: -z with -a multi-argument mode.
		{
			Name: "zero_multi",
			Args: []string{"-z", "-a", "/usr/bin/sort", "/usr/bin/ls"},
		},
		// R4.2: --zero long form.
		{
			Name: "zero_long",
			Args: []string{"--zero", "foo"},
		},
		// R4.2: combined -a -s -z flags.
		{
			Name: "combined_multi_suffix_zero",
			Args: []string{"-a", "-s", ".h", "-z", "include/stdio.h", "include/stdlib.h"},
		},
		// R4.3: no arguments — error, exit 1.
		{
			Name:      "no_args_error",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3: extra operand in single-argument mode — error, exit 1.
		{
			Name:      "extra_operand_error",
			Args:      []string{"a", "b", "c"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3: invalid option — error, exit 1.
		{
			Name:      "invalid_option",
			Args:      []string{"--invalid-option"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// --help output.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		// --version output.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		// -- separator followed by names.
		{
			Name: "double_dash",
			Args: []string{"-a", "--", "/usr/bin/sort"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
