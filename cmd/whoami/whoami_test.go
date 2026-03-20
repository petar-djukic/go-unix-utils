// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd042-whoami R3.1–R3.3: differential tests for whoami.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes stderr differences between Go and GNU whoami:
// replaces binary name/path at the start of lines with "whoami" and strips
// "Try ... for more information." lines.
var stderrNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	tryRe := regexp.MustCompile(`(?m)^Try .* for more information\.\n`)
	data = tryRe.ReplaceAll(data, nil)
	nameRe := regexp.MustCompile(`(?m)^[^\s:]+`)
	data = nameRe.ReplaceAll(data, []byte("whoami"))
	// Normalize GNU getopt "invalid option -- 'X'" to Go format.
	optRe := regexp.MustCompile(`invalid option -- '(.)'`)
	data = optRe.ReplaceAll(data, []byte("unrecognized option '-$1'"))
	return data
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwhoami")
	if err != nil {
		t.Skipf("reference binary gwhoami not in PATH: %v", err)
	}
	errNorm := []testutils.NormalizeFunc{stderrNormalizer}
	tests := []testutils.DiffTest{
		{
			// R3.2: normal invocation with no arguments.
			Name:     "no_args",
			Args:     []string{},
			ExitCode: 0,
		},
		{
			// R3.2: extra operand error.
			Name:      "extra_operand",
			Args:      []string{"extraarg"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			// R3.2: unknown flag error.
			Name:      "unknown_flag",
			Args:      []string{"--bogus"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			// R3.2: short unknown flag error.
			Name:      "unknown_short_flag",
			Args:      []string{"-x"},
			ExitCode:  1,
			Normalize: errNorm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
