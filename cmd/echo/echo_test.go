// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd020-echo R1.1–R1.4: core output behavior.
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
		// R1.2: no arguments outputs only a newline
		{
			Name: "no_args",
			Args: []string{},
		},
		// R1.1: single argument followed by newline
		{
			Name: "single_arg",
			Args: []string{"hello"},
		},
		// R1.1: multiple arguments joined by spaces
		{
			Name: "multiple_args",
			Args: []string{"hello", "world"},
		},
		// R1.1: three arguments
		{
			Name: "three_args",
			Args: []string{"a", "b", "c"},
		},
		// R1.1: empty string argument
		{
			Name: "empty_string_arg",
			Args: []string{""},
		},
		// R1.1: multiple empty string arguments
		{
			Name: "multiple_empty_args",
			Args: []string{"", "", ""},
		},
		// R1.3: -n suppresses trailing newline
		{
			Name: "flag_n",
			Args: []string{"-n", "hello"},
		},
		// R1.3: -n with multiple arguments
		{
			Name: "flag_n_multiple_args",
			Args: []string{"-n", "hello", "world"},
		},
		// R1.3: -n with no other arguments
		{
			Name: "flag_n_no_args",
			Args: []string{"-n"},
		},
		// R1.3: combined -n with recognized flags
		{
			Name: "flag_nE",
			Args: []string{"-nE", "hello"},
		},
		// R1.3: -n repeated
		{
			Name: "flag_nn",
			Args: []string{"-nn", "hello"},
		},
		// R1.4: unrecognized flag treated as literal text
		{
			Name: "unrecognized_flag_x",
			Args: []string{"-x", "hello"},
		},
		// R1.4: -a is not a recognized flag
		{
			Name: "unrecognized_flag_a",
			Args: []string{"-a"},
		},
		// R1.4: unrecognized flag stops flag parsing
		{
			Name: "unrecognized_stops_parsing",
			Args: []string{"-a", "-n"},
		},
		// R1.4: argument starting with dash that mixes recognized and unrecognized
		{
			Name: "mixed_flag_chars",
			Args: []string{"-nq", "hello"},
		},
		// R1.1: argument with spaces
		{
			Name: "arg_with_spaces",
			Args: []string{"hello world"},
		},
		// R1.1: argument containing special characters
		{
			Name: "special_chars",
			Args: []string{"hello\tworld"},
		},
		// R1.4: bare dash is not a flag
		{
			Name: "bare_dash",
			Args: []string{"-"},
		},
		// R1.3: multiple -n flags
		{
			Name: "multiple_n_flags",
			Args: []string{"-n", "-n", "hello"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
