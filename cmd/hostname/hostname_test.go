// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd047-hostname R3.1 (compare Go vs ghostname),
// R3.2 (normal invocation, extra operand, unknown flag),
// R3.3 (LC_ALL=C in test environment).
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes stderr differences between Go and GNU hostname:
// replaces binary name/path variants with "hostname" and strips
// "Try ... for more information." lines.
var stderrNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	nameRe := regexp.MustCompile(`[^\s']*g?hostname`)
	data = nameRe.ReplaceAll(data, []byte("hostname"))
	tryRe := regexp.MustCompile(`(?m)^Try .* for more information\.\n`)
	data = tryRe.ReplaceAll(data, nil)
	return data
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ghostname")
	if err != nil {
		t.Skipf("reference binary ghostname not in PATH: %v", err)
	}
	env := []string{"LC_ALL=C"}
	errNorm := []testutils.NormalizeFunc{stderrNormalizer}
	tests := []testutils.DiffTest{
		// R3.1, R3.2: default invocation — prints system hostname.
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
		// R3.2 edge case: double-dash separator followed by extra operand — error exit 1.
		{
			Name:      "double_dash_extra_operand",
			Args:      []string{"--", "extraarg"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R3.2 edge case: multiple extra operands — error on first.
		{
			Name:      "multiple_extra_operands",
			Args:      []string{"arg1", "arg2"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R3.2 edge case: double-dash alone — treated as no args, prints hostname.
		{
			Name: "double_dash_alone",
			Args: []string{"--"},
			Env:  env,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
