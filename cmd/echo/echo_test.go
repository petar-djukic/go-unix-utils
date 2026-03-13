// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/echo against gecho.
// Implements: prd020-echo R4.1, R4.2, R4.3
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gecho")
	if err != nil {
		t.Skipf("reference binary gecho not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.2: No arguments prints only a newline.
		{
			Name: "no arguments",
			Args: []string{},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: Single argument followed by newline.
		{
			Name: "single argument",
			Args: []string{"hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: Multiple arguments separated by spaces.
		{
			Name: "multiple arguments",
			Args: []string{"hello", "world"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: -n suppresses trailing newline.
		{
			Name: "suppress newline",
			Args: []string{"-n", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: -n with multiple arguments.
		{
			Name: "suppress newline multiple args",
			Args: []string{"-n", "hello", "world"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: Unrecognized flag treated as positional.
		{
			Name: "unrecognized flag as literal",
			Args: []string{"-z", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: Bare dash is a positional argument.
		{
			Name: "dash alone is literal",
			Args: []string{"-"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: Mixed valid/invalid flag chars treated as literal.
		{
			Name: "mixed valid and invalid flag chars",
			Args: []string{"-nz", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: Double dash is not special in echo; treated as literal.
		{
			Name: "double dash is literal",
			Args: []string{"--", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: Flag-like arg after positional is literal.
		{
			Name: "flag after positional is literal",
			Args: []string{"hello", "-n"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: Empty string argument.
		{
			Name: "empty string argument",
			Args: []string{""},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: Multiple empty strings produce spaces.
		{
			Name: "multiple empty strings",
			Args: []string{"", "", ""},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: -n with no further arguments.
		{
			Name: "suppress newline no args",
			Args: []string{"-n"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3, R1.4: Combined flag -nE recognized.
		{
			Name: "combined nE flag",
			Args: []string{"-nE", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
