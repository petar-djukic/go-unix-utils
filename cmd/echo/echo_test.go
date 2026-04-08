// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/echo. Implements srd020-echo R4.1, R4.2, R4.3.
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

	// R4.2: tests cover no arguments, single argument, multiple arguments,
	// -n flag, -e with each supported escape sequence, -E flag, combined
	// -n -e, and arguments starting with '-'.
	// R4.3: LC_ALL=C is set by default via testutils.RunDiffTests buildEnv.
	tests := []testutils.DiffTest{
		// R1.2: no arguments produces only a newline.
		{
			Name: "no_arguments",
			Args: []string{},
		},
		// R1.1: single argument followed by newline.
		{
			Name: "single_argument",
			Args: []string{"hello"},
		},
		// R1.1: multiple arguments joined by spaces.
		{
			Name: "multiple_arguments",
			Args: []string{"hello", "world"},
		},
		// R1.3: -n suppresses trailing newline.
		{
			Name: "flag_n",
			Args: []string{"-n", "hello"},
		},
		// R1.3: -n with no arguments.
		{
			Name: "flag_n_no_args",
			Args: []string{"-n"},
		},
		// R2.3: -E disables escapes (default behavior).
		{
			Name: "flag_E_literal_backslash",
			Args: []string{"-E", `hello\nworld`},
		},
		// R2.1: -e backslash escapes: \\ (backslash).
		{
			Name: "escape_backslash",
			Args: []string{"-e", `a\\b`},
		},
		// R2.1: -e \a (alert/BEL).
		{
			Name: "escape_alert",
			Args: []string{"-e", `\a`},
		},
		// R2.1: -e \b (backspace).
		{
			Name: "escape_backspace",
			Args: []string{"-e", `\b`},
		},
		// R2.1: -e \e (escape char 0x1B).
		{
			Name: "escape_esc",
			Args: []string{"-e", `\e`},
		},
		// R2.1: -e \f (form feed).
		{
			Name: "escape_formfeed",
			Args: []string{"-e", `\f`},
		},
		// R2.1: -e \n (newline).
		{
			Name: "escape_newline",
			Args: []string{"-e", `a\nb`},
		},
		// R2.1: -e \r (carriage return).
		{
			Name: "escape_carriage_return",
			Args: []string{"-e", `a\rb`},
		},
		// R2.1: -e \t (horizontal tab).
		{
			Name: "escape_tab",
			Args: []string{"-e", `a\tb`},
		},
		// R2.1: -e \v (vertical tab).
		{
			Name: "escape_vertical_tab",
			Args: []string{"-e", `\v`},
		},
		// R2.1: -e \0NNN (octal).
		{
			Name: "escape_octal",
			Args: []string{"-e", `\0101`},
		},
		// R2.1: -e \xHH (hex).
		{
			Name: "escape_hex",
			Args: []string{"-e", `\x41`},
		},
		// R2.2: -e \c terminates output immediately.
		{
			Name: "escape_c_stops_output",
			Args: []string{"-e", `before\cafter`},
		},
		// R2.2: \c with multiple arguments stops all further output.
		{
			Name: "escape_c_multiple_args",
			Args: []string{"-e", `first\c`, "second"},
		},
		// R2.4: last of -e/-E wins (here -E last, escapes disabled).
		{
			Name: "flag_e_then_E",
			Args: []string{"-e", "-E", `hello\nworld`},
		},
		// R2.4: last of -e/-E wins (here -e last, escapes enabled).
		{
			Name: "flag_E_then_e",
			Args: []string{"-E", "-e", `hello\nworld`},
		},
		// R1.3 + R2.1: combined -n -e.
		{
			Name: "combined_n_e",
			Args: []string{"-n", "-e", `hello\tworld`},
		},
		// Combined flags in single argument: -ne.
		{
			Name: "combined_ne_single",
			Args: []string{"-ne", `hello\n`},
		},
		// R1.4: unrecognized flags treated as positional arguments.
		{
			Name: "unrecognized_flag_dash_x",
			Args: []string{"-x", "hello"},
		},
		// R1.4: argument starting with '-' that is not a flag.
		{
			Name: "dash_dash_literal",
			Args: []string{"--", "hello"},
		},
		// R1.4: mixed valid and invalid flag chars.
		{
			Name: "flag_mix_invalid",
			Args: []string{"-nz", "hello"},
		},
		// Multiple escape sequences in one argument.
		{
			Name: "multiple_escapes",
			Args: []string{"-e", `\a\b\t\n`},
		},
		// Empty string argument.
		{
			Name: "empty_string_arg",
			Args: []string{""},
		},
		// Multiple empty string arguments.
		{
			Name: "multiple_empty_args",
			Args: []string{"", "", ""},
		},
	}

	// R4.1: compare Go echo output against gecho byte-for-byte.
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
