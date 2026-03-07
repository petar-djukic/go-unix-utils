// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/echo against the GNU reference binary (gecho).
//
// Implements prd020-echo acceptance criteria AC1-AC5 via testutils.RunDiffTests.
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
		// R1.1: Arguments joined with spaces, trailing newline.
		{
			Name: "echo_default_output",
			Args: []string{"hello", "world"},
		},
		// R1.2: No arguments, only a newline.
		{
			Name: "echo_no_args",
			Args: []string{},
		},
		// R1.3: -n suppresses trailing newline.
		{
			Name: "echo_suppress_newline",
			Args: []string{"-n", "hello"},
		},
		// R1.4: Unrecognized flag treated as positional argument.
		{
			Name: "echo_unrecognized_flag_literal",
			Args: []string{"-z", "hello"},
		},
		// R1.4: Argument starting with - but mixed valid/invalid chars.
		{
			Name: "echo_mixed_flag_literal",
			Args: []string{"-abc", "hello"},
		},
		// R2.1: -e interprets \t.
		{
			Name: "echo_escape_tab",
			Args: []string{"-e", `col1\tcol2`},
		},
		// R2.1: -e interprets \n.
		{
			Name: "echo_escape_newline",
			Args: []string{"-e", `line1\nline2`},
		},
		// R2.1: -e interprets \\ (literal backslash).
		{
			Name: "echo_escape_backslash",
			Args: []string{"-e", `a\\b`},
		},
		// R2.1: -e interprets \a (BEL).
		{
			Name: "echo_escape_alert",
			Args: []string{"-e", `x\ay`},
		},
		// R2.1: -e interprets \b (backspace).
		{
			Name: "echo_escape_backspace",
			Args: []string{"-e", `x\by`},
		},
		// R2.1: -e interprets \e (ESC).
		{
			Name: "echo_escape_esc",
			Args: []string{"-e", `x\ey`},
		},
		// R2.1: -e interprets \f (form feed).
		{
			Name: "echo_escape_formfeed",
			Args: []string{"-e", `x\fy`},
		},
		// R2.1: -e interprets \r (carriage return).
		{
			Name: "echo_escape_cr",
			Args: []string{"-e", `x\ry`},
		},
		// R2.1: -e interprets \v (vertical tab).
		{
			Name: "echo_escape_vtab",
			Args: []string{"-e", `x\vy`},
		},
		// R2.1: -e interprets \0NNN (octal).
		{
			Name: "echo_escape_octal",
			Args: []string{"-e", `\0101`},
		},
		// R2.1: -e interprets \xHH (hex).
		{
			Name: "echo_escape_hex",
			Args: []string{"-e", `\x41`},
		},
		// R2.2: \c with -e terminates output immediately.
		{
			Name: "echo_escape_c_terminates",
			Args: []string{"-e", `before\cafter`},
		},
		// R2.2: \c stops output across arguments too.
		{
			Name: "echo_escape_c_stops_remaining_args",
			Args: []string{"-e", `first\c`, "second"},
		},
		// R2.3: Without -e, backslashes are literal.
		{
			Name: "echo_no_escape_default",
			Args: []string{`a\tb`},
		},
		// R2.3: -E explicitly disables escapes.
		{
			Name: "echo_explicit_E",
			Args: []string{"-E", `a\tb`},
		},
		// R2.4: -e -E combined, last wins (E wins).
		{
			Name: "echo_e_then_E",
			Args: []string{"-e", "-E", `a\tb`},
		},
		// R2.4: -E -e combined, last wins (e wins).
		{
			Name: "echo_E_then_e",
			Args: []string{"-E", "-e", `a\tb`},
		},
		// R2.4: -eE in a single arg, E is last so escapes disabled.
		{
			Name: "echo_eE_single_arg",
			Args: []string{"-eE", `a\tb`},
		},
		// R2.4: -Ee in a single arg, e is last so escapes enabled.
		{
			Name: "echo_Ee_single_arg",
			Args: []string{"-Ee", `a\tb`},
		},
		// Combined -n -e.
		{
			Name: "echo_combined_n_e",
			Args: []string{"-n", "-e", `x\ty`},
		},
		// Combined -ne in single arg.
		{
			Name: "echo_combined_ne_single",
			Args: []string{"-ne", `x\ty`},
		},
		// Single argument.
		{
			Name: "echo_single_arg",
			Args: []string{"hello"},
		},
		// Multiple arguments with spaces.
		{
			Name: "echo_multiple_args",
			Args: []string{"a", "b", "c", "d"},
		},
		// Argument that looks like a flag after a non-flag arg.
		{
			Name: "echo_dash_after_positional",
			Args: []string{"hello", "-n"},
		},
		// Empty string argument.
		{
			Name: "echo_empty_string_arg",
			Args: []string{""},
		},
		// -n with no other arguments.
		{
			Name: "echo_n_no_args",
			Args: []string{"-n"},
		},
		// Octal \0 with no digits = NUL byte.
		{
			Name: "echo_escape_octal_zero",
			Args: []string{"-e", `\0`},
		},
		// Hex with single digit.
		{
			Name: "echo_escape_hex_single_digit",
			Args: []string{"-e", `\x9`},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
