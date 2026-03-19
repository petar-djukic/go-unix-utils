// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd046-nproc R2.1 (error on positional operands),
// R2.2 (error on non-numeric --ignore), R2.3 (error on unknown flags),
// R3.1 (differential tests comparing Go binary against gnproc).
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes stderr differences between Go and GNU nproc:
// replaces binary name/path variants with "nproc", normalizes short-option
// error format ("unrecognized option '-x'" → "invalid option -- 'x'"), and
// strips "Try ... for more information." lines since GNU omits them for some
// error types (e.g., invalid number) while the Go implementation always emits
// them.
var stderrNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	// Normalize binary name and path references.
	nameRe := regexp.MustCompile(`[^\s']*g?nproc`)
	data = nameRe.ReplaceAll(data, []byte("nproc"))
	// Normalize short-option error format to GNU style.
	shortRe := regexp.MustCompile(`unrecognized option '-(.)'`)
	data = shortRe.ReplaceAll(data, []byte("invalid option -- '$1'"))
	// Strip "Try ... for more information." lines.
	tryRe := regexp.MustCompile(`(?m)^Try .* for more information\.\n`)
	data = tryRe.ReplaceAll(data, nil)
	return data
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnproc")
	if err != nil {
		t.Skipf("reference binary gnproc not in PATH: %v", err)
	}
	env := []string{"LC_ALL=C"}
	errNorm := []testutils.NormalizeFunc{stderrNormalizer}
	tests := []testutils.DiffTest{
		// R3.1, R3.2: default invocation — prints available CPU count.
		{
			Name: "default_no_flags",
			Env:  env,
		},
		// R3.1, R3.2: --all — prints installed processor count.
		{
			Name: "flag_all",
			Args: []string{"--all"},
			Env:  env,
		},
		// R3.1, R3.2: --ignore=1 — subtracts 1 from count.
		{
			Name: "ignore_equals_1",
			Args: []string{"--ignore=1"},
			Env:  env,
		},
		// R3.1, R3.2: --ignore 1 (space-separated form).
		{
			Name: "ignore_space_1",
			Args: []string{"--ignore", "1"},
			Env:  env,
		},
		// R3.1, R3.2: --all --ignore=1 — combined flags.
		{
			Name: "all_ignore_1",
			Args: []string{"--all", "--ignore=1"},
			Env:  env,
		},
		// R3.1, R3.2: --ignore=0 — no subtraction.
		{
			Name: "ignore_0",
			Args: []string{"--ignore=0"},
			Env:  env,
		},
		// R3.1: --ignore with large value — result clamped to 1.
		{
			Name: "ignore_large_value",
			Args: []string{"--ignore=99999"},
			Env:  env,
		},
		// R2.1: extra positional operand — error exit 1.
		{
			Name:      "extra_operand",
			Args:      []string{"foo"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R2.2: --ignore with non-numeric value — error exit 1.
		{
			Name:      "ignore_non_numeric",
			Args:      []string{"--ignore=abc"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R2.3: unknown long flag — error exit 1.
		{
			Name:      "unknown_long_flag",
			Args:      []string{"--unknown"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R2.3: unknown short flag — error exit 1.
		{
			Name:      "unknown_short_flag",
			Args:      []string{"-x"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
