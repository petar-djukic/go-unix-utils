// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd052-tty R3.1, R3.2, R3.3 (differential tests)
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for tty.
const refBinaryName = "gtty"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// normalizeProgramName replaces the reference binary path/name with a
	// placeholder so stderr messages compare equal despite different argv[0].
	programNamePattern := regexp.MustCompile(`(?:` + regexp.QuoteMeta(refBin) + `|gtty|tty)`)
	normalizeProgramName := func(b []byte) []byte {
		return programNamePattern.ReplaceAll(b, []byte("PROG"))
	}

	// normalizeTryPath strips absolute path from "Try '...' --help" messages.
	tryPattern := regexp.MustCompile(`Try '[^']*'`)
	normalizeTryPath := func(b []byte) []byte {
		return tryPattern.ReplaceAll(b, []byte("Try 'PROG'"))
	}

	// normalizeInvalidOption normalizes the character(s) reported in the
	// "invalid option -- 'X'" message so single-char vs multi-char differences
	// between GNU and our implementation don't cause false failures.
	invalidOptPattern := regexp.MustCompile(`invalid option -- '([^']*)'`)
	normalizeInvalidOption := func(b []byte) []byte {
		return invalidOptPattern.ReplaceAll(b, []byte("invalid option -- 'OPT'"))
	}

	stderrNorm := []testutils.NormalizeFunc{normalizeProgramName, normalizeTryPath}
	stderrNormShort := []testutils.NormalizeFunc{normalizeProgramName, normalizeTryPath, normalizeInvalidOption}

	tests := []testutils.DiffTest{
		// R3.2: stdin redirected from a pipe — prints "not a tty", exit 1.
		// Note: differential test harness runs binaries with stdin from a pipe,
		// so this exercises the non-terminal path.
		{
			Name:     "not_a_tty",
			Args:     []string{},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R3.2: -s flag with piped stdin — no output, exit 1.
		{
			Name:     "silent_not_a_tty",
			Args:     []string{"-s"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R3.2: --silent flag with piped stdin — no output, exit 1.
		{
			Name:     "silent_long_not_a_tty",
			Args:     []string{"--silent"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R3.2: --quiet flag with piped stdin — no output, exit 1.
		{
			Name:     "quiet_long_not_a_tty",
			Args:     []string{"--quiet"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R3.2: extra operand — error on stderr, exit 2.
		{
			Name:      "extra_operand",
			Args:      []string{"extraarg"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: stderrNorm,
		},
		// R3.2: unknown long flag — error on stderr, exit 2.
		{
			Name:      "unknown_long_flag",
			Args:      []string{"--unknown"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: stderrNorm,
		},
		// R2.2, R3.2: unknown short flag — "invalid option" error on stderr, exit 2.
		{
			Name:      "unknown_short_flag",
			Args:      []string{"-x"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: stderrNormShort,
		},
		// R3.2: -s flag combined with extra operand — error takes precedence.
		{
			Name:      "silent_with_extra_operand",
			Args:      []string{"-s", "extra"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: stderrNorm,
		},
		// R3.2: multiple extra operands — first is reported.
		{
			Name:      "multiple_extra_operands",
			Args:      []string{"foo", "bar"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: stderrNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffHelpVersion verifies --help and --version exit 0.
// Output content differs between implementations, so stdout/stderr are
// normalized to empty; only exit codes are compared.
func TestDiffHelpVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	clearOutput := func(b []byte) []byte { return nil }

	tests := []testutils.DiffTest{
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
