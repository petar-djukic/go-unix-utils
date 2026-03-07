// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/echo against gecho reference binary.
// Implements prd020-echo R1-R3.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gecho")
	if err != nil {
		t.Skipf("reference binary gecho not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: Arguments joined with spaces, followed by newline.
		{
			Name: "default_output",
			Args: []string{"hello", "world"},
		},
		// R1.2: No arguments outputs only a newline.
		{
			Name: "no_args",
			Args: []string{},
		},
		// R1.3: -n suppresses trailing newline.
		{
			Name: "suppress_newline",
			Args: []string{"-n", "hello"},
		},
		// R2.1: -e interprets \t as horizontal tab.
		{
			Name: "escape_tab",
			Args: []string{"-e", `col1\tcol2`},
		},
		// R2.1: -e interprets \n as embedded newline.
		{
			Name: "escape_newline",
			Args: []string{"-e", `line1\nline2`},
		},
		// R2.2: -e with \c terminates output; no trailing newline.
		{
			Name: "escape_c_terminates",
			Args: []string{"-e", `before\cafter`},
		},
		// R2.3: Without -e, backslash sequences are literal.
		{
			Name: "no_escape_default",
			Args: []string{`a\tb`},
		},
		// R1.3, R2.1: -n -e combined.
		{
			Name: "combined_n_e",
			Args: []string{"-n", "-e", `x\ty`},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
