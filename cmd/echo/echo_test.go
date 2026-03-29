// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff verifies cmd/echo against the GNU reference binary gecho.
// Implements prd020-echo R4.1-R4.3.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath("gecho")
	if err != nil {
		t.Skipf("reference binary gecho not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: Arguments joined by spaces, followed by newline.
		{
			Name: "single_argument",
			Args: []string{"hello"},
		},
		{
			Name: "multiple_arguments",
			Args: []string{"hello", "world"},
		},
		{
			Name: "three_arguments",
			Args: []string{"a", "b", "c"},
		},
		// R1.2: No arguments produces only a newline.
		{
			Name: "no_arguments",
			Args: []string{},
		},
		// R1.3: -n suppresses trailing newline.
		{
			Name: "flag_n_single_arg",
			Args: []string{"-n", "hello"},
		},
		{
			Name: "flag_n_multiple_args",
			Args: []string{"-n", "hello", "world"},
		},
		{
			Name: "flag_n_no_args",
			Args: []string{"-n"},
		},
		// R1.4: Unrecognized flags passed as literal text.
		{
			Name: "unrecognized_flag_literal",
			Args: []string{"-x", "hello"},
		},
		{
			Name: "double_dash_literal",
			Args: []string{"--", "hello"},
		},
		{
			Name: "dash_only_literal",
			Args: []string{"-"},
		},
		{
			Name: "mixed_valid_invalid_flag_chars",
			Args: []string{"-nz", "hello"},
		},
		// R1.4: Combined recognized flags.
		{
			Name: "combined_nE_flag",
			Args: []string{"-nE", "hello"},
		},
		{
			Name: "combined_ne_flag",
			Args: []string{"-ne", "hello"},
		},
		// R1.4: -e and -E are recognized flags (not literal).
		{
			Name: "flag_e_alone",
			Args: []string{"-e", "hello"},
		},
		{
			Name: "flag_E_alone",
			Args: []string{"-E", "hello"},
		},
		// R1.4: Multiple separate flag args.
		{
			Name: "separate_n_e_flags",
			Args: []string{"-n", "-e", "hello"},
		},
		// R1.1: Arguments with special characters passed literally.
		{
			Name: "argument_with_spaces",
			Args: []string{"hello world"},
		},
		{
			Name: "empty_string_argument",
			Args: []string{""},
		},
		{
			Name: "multiple_empty_arguments",
			Args: []string{"", ""},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
