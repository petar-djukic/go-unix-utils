// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd052-tty R3.1 (compare Go vs gtty),
// R3.2 (piped stdin, -s flag, error cases),
// R3.3 (LC_ALL=C in test environment).
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes stderr differences between Go and GNU tty:
// replaces binary name/path variants with "tty" and strips
// "Try ... for more information." lines.
var stderrNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	nameRe := regexp.MustCompile(`[^\s']*g?tty`)
	data = nameRe.ReplaceAll(data, []byte("tty"))
	tryRe := regexp.MustCompile(`(?m)^Try .* for more information\.\n`)
	data = tryRe.ReplaceAll(data, nil)
	return data
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtty")
	if err != nil {
		t.Skipf("reference binary gtty not in PATH: %v", err)
	}
	env := []string{"LC_ALL=C"}
	errNorm := []testutils.NormalizeFunc{stderrNormalizer}
	tests := []testutils.DiffTest{
		// R1.2, R3.1: piped stdin — prints "not a tty", exit 1.
		{
			Name:     "piped_stdin_not_a_tty",
			Stdin:    []byte{},
			Env:      env,
			ExitCode: 1,
		},
		// R1.3, R3.2: -s flag with piped stdin — no output, exit 1.
		{
			Name:     "silent_short_flag",
			Args:     []string{"-s"},
			Stdin:    []byte{},
			Env:      env,
			ExitCode: 1,
		},
		// R1.3: --silent long flag with piped stdin — no output, exit 1.
		{
			Name:     "silent_long_flag",
			Args:     []string{"--silent"},
			Stdin:    []byte{},
			Env:      env,
			ExitCode: 1,
		},
		// R1.3: --quiet long flag with piped stdin — no output, exit 1.
		{
			Name:     "quiet_long_flag",
			Args:     []string{"--quiet"},
			Stdin:    []byte{},
			Env:      env,
			ExitCode: 1,
		},
		// R2.1: extra operand — error, exit 2.
		{
			Name:      "extra_operand",
			Args:      []string{"extraarg"},
			Env:       env,
			ExitCode:  2,
			Normalize: errNorm,
		},
		// R2.2: unknown long flag — error, exit 2.
		{
			Name:      "unknown_long_flag",
			Args:      []string{"--unknown"},
			Env:       env,
			ExitCode:  2,
			Normalize: errNorm,
		},
		// R2.2: unknown short flag — error, exit 2.
		{
			Name:      "unknown_short_flag",
			Args:      []string{"-x"},
			Env:       env,
			ExitCode:  2,
			Normalize: errNorm,
		},
		// Edge case: double-dash then extra operand — error, exit 2.
		{
			Name:      "double_dash_extra_operand",
			Args:      []string{"--", "extraarg"},
			Env:       env,
			ExitCode:  2,
			Normalize: errNorm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
