// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tty against gtty (GNU coreutils).
// Implements prd052-tty R1.1-R1.3, R2.1-R2.2, R3.1-R3.3 test coverage.
package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	// D3: graceful skip if gtty is not installed.
	refBin, err := exec.LookPath("gtty")
	if err != nil {
		t.Skipf("reference binary gtty not in PATH: %v", err)
	}

	// D4: normalizer that strips stderr for error message format differences.
	normalizeStderr := func(b []byte) []byte {
		return nil
	}

	// Normalizer for --version output: keep only the program name.
	normalizeVersion := func(b []byte) []byte {
		if i := bytes.IndexByte(b, '\n'); i >= 0 {
			b = b[:i+1]
		}
		if i := bytes.IndexByte(b, ' '); i >= 0 {
			return append(b[:i], '\n')
		}
		return b
	}

	// Normalizer for --help output: reduce to fixed token.
	normalizeHelp := func(b []byte) []byte {
		if len(b) > 0 {
			return []byte("help\n")
		}
		return b
	}

	tests := []testutils.DiffTest{
		// R1.2: stdin from pipe → "not a tty", exit 1.
		// RunDiffTests pipes stdin, so stdin is never a terminal.
		{
			Name:     "R1.2_not_a_tty",
			Args:     []string{},
			ExitCode: 1,
		},
		// R1.3: -s flag, stdin from pipe → no output, exit 1.
		{
			Name:     "R1.3_silent_short",
			Args:     []string{"-s"},
			ExitCode: 1,
		},
		// R1.3: --silent long flag.
		{
			Name:     "R1.3_silent_long",
			Args:     []string{"--silent"},
			ExitCode: 1,
		},
		// R1.3: --quiet long flag.
		{
			Name:     "R1.3_quiet_long",
			Args:     []string{"--quiet"},
			ExitCode: 1,
		},
		// R2.1: extra operand → error, exit 2.
		{
			Name:      "R2.1_extra_operand",
			Args:      []string{"extraarg"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R2.1: extra operand after -s → error, exit 2.
		{
			Name:      "R2.1_extra_operand_after_silent",
			Args:      []string{"-s", "extraarg"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R2.2: unknown short flag → error, exit 2.
		{
			Name:      "R2.2_unknown_short_flag",
			Args:      []string{"-x"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R2.2: unknown long flag → error, exit 2.
		{
			Name:      "R2.2_unknown_long_flag",
			Args:      []string{"--foobar"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// --version prints version info to stdout, exit 0.
		{
			Name:      "version",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeVersion},
		},
		// --help prints usage to stdout, exit 0.
		{
			Name:      "help",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeHelp},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
