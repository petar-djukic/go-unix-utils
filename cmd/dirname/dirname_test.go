// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/dirname against GNU gdirname.
// Covers prd016-dirname R4.1-R4.3 (differential testing).
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
	binPath := regexp.MustCompile(`(?m)/[^\s]+/g?dirname`)
	versionLine := regexp.MustCompile(`(?m)^dirname \([^)]+\) .+$`)
	gnuTrailer := regexp.MustCompile(`(?m)^(Copyright|License|Written by|This is free|There is NO|Report |General help|or available|Full documentation|GNU coreutils).*\n?`)
	optWrap := regexp.MustCompile(`(--(?:help|version|zero))\n\s+`)
	optSpace := regexp.MustCompile(`(--(?:help|version|zero))\s{2,}`)
	blankLine := regexp.MustCompile(`\n\n(\s+--(?:help|version))`)
	// Strip Examples section (may differ between Go and GNU).
	examplesBlock := regexp.MustCompile(`(?ms)\nExamples:\n.*`)
	return func(b []byte) []byte {
		b = ansiEsc.ReplaceAll(b, nil)
		b = binPath.ReplaceAll(b, []byte("dirname"))
		b = versionLine.ReplaceAll(b, []byte("dirname (NORMALIZED) VERSION"))
		b = gnuTrailer.ReplaceAll(b, nil)
		b = examplesBlock.ReplaceAll(b, nil)
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
	binPath := regexp.MustCompile(`/[^\s:]+/g?dirname|gdirname`)
	tryHelp := regexp.MustCompile(`(?m)^Try '[^']*' for more information\.\n?`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("dirname"))
		b = tryHelp.ReplaceAll(b, nil)
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdirname")
	if err != nil {
		t.Skipf("reference binary gdirname not in PATH: %v", err)
	}

	norm := helpVersionNormalizer()
	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		// R4.1/R4.2: simple path — strip last component.
		{
			Name: "simple_path",
			Args: []string{"/usr/bin/sort"},
		},
		// R4.2: nested path.
		{
			Name: "nested_path",
			Args: []string{"a/b/c"},
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
		// R4.2: relative path with no directory component.
		{
			Name: "no_directory",
			Args: []string{"file.txt"},
		},
		// R4.2: dot path.
		{
			Name: "dot_path",
			Args: []string{"."},
		},
		// R4.2: double-dot path.
		{
			Name: "double_dot_path",
			Args: []string{".."},
		},
		// R4.2: multiple arguments.
		{
			Name: "multiple_args",
			Args: []string{"/usr/bin/sort", "/usr/lib/libc.so"},
		},
		// R4.2: NUL-delimited output with -z flag.
		{
			Name: "zero_flag",
			Args: []string{"-z", "/usr/bin/sort"},
		},
		// R4.2: --zero long form.
		{
			Name: "zero_long",
			Args: []string{"--zero", "/usr/bin/sort"},
		},
		// R4.2: -z with multiple arguments.
		{
			Name: "zero_multi",
			Args: []string{"-z", "/usr/bin/sort", "/usr/lib/libc.so"},
		},
		// R4.2: path with trailing slash on directory.
		{
			Name: "dir_trailing_slash",
			Args: []string{"/usr/bin/"},
		},
		// R4.3: no arguments — error, exit 1.
		{
			Name:      "no_args_error",
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
			Args: []string{"--", "/usr/bin/sort"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
