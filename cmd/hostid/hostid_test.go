// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd048-hostid R3.1 (compare Go vs ghostid),
// R3.2 (normal invocation, extra operand, unknown flag),
// R3.3 (LC_ALL=C in test environment).
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes stderr differences between Go and GNU hostid:
// replaces binary name/path variants with "hostid", strips
// "Try ... for more information." lines, and normalizes error phrasing.
var stderrNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	nameRe := regexp.MustCompile(`[^\s']*g?hostid`)
	data = nameRe.ReplaceAll(data, []byte("hostid"))
	tryRe := regexp.MustCompile(`(?m)^Try .* for more information\.\n`)
	data = tryRe.ReplaceAll(data, nil)
	// Normalize short-flag error phrasing: GNU uses "invalid option -- 'x'",
	// Go uses "unrecognized option '-x'".
	shortRe := regexp.MustCompile(`invalid option -- '(.)'`)
	data = shortRe.ReplaceAll(data, []byte("unrecognized option '-$1'"))
	return data
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ghostid")
	if err != nil {
		t.Skipf("reference binary ghostid not in PATH: %v", err)
	}
	env := []string{"LC_ALL=C"}
	errNorm := []testutils.NormalizeFunc{stderrNormalizer}
	tests := []testutils.DiffTest{
		// R3.1, R3.2: default invocation — prints host identifier.
		{
			Name: "default_no_args",
			Env:  env,
		},
		// R3.2, R2.1: extra operand — error exit 1.
		{
			Name:      "extra_operand",
			Args:      []string{"extraarg"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R3.2, R2.2: unknown long flag — error exit 1.
		{
			Name:      "unknown_long_flag",
			Args:      []string{"--unknown"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R3.2, R2.2: unknown short flag — error exit 1.
		{
			Name:      "unknown_short_flag",
			Args:      []string{"-x"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R3.2: double-dash separator alone — succeeds, prints hostid.
		{
			Name: "double_dash_only",
			Args: []string{"--"},
			Env:  env,
		},
		// R3.2, R2.1: double-dash followed by extra operand — error exit 1.
		{
			Name:      "double_dash_extra_operand",
			Args:      []string{"--", "extraarg"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R3.2, R2.1: multiple extra operands — error on first.
		{
			Name:      "multiple_extra_operands",
			Args:      []string{"foo", "bar"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
