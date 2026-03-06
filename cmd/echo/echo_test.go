// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/echo (prd020-echo R4).
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests comparing Go echo against gecho.
// R4.1: byte-for-byte comparison via RunDiffTests.
// R4.3: LC_ALL=C set via DiffTest.Env.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// D4: Graceful skip if gecho is not in PATH.
	refBin, err := exec.LookPath("gecho")
	if err != nil {
		t.Skipf("reference binary gecho not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: Arguments joined with spaces, followed by newline.
		{
			Name: "echo_default_output",
			Args: []string{"hello", "world"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: No arguments outputs only a newline.
		{
			Name: "echo_no_args",
			Args: []string{},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: -n suppresses trailing newline.
		{
			Name: "echo_suppress_newline",
			Args: []string{"-n", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e interprets \t as horizontal tab.
		{
			Name: "echo_escape_tab",
			Args: []string{"-e", `col1\tcol2`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e interprets \n as embedded newline.
		{
			Name: "echo_escape_newline",
			Args: []string{"-e", `line1\nline2`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: -e with \c terminates output immediately.
		{
			Name: "echo_escape_c_terminates",
			Args: []string{"-e", `before\cafter`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: Without -e, backslash sequences are literal.
		{
			Name: "echo_no_escape_default",
			Args: []string{`a\tb`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: Explicit -E disables escapes.
		{
			Name: "echo_explicit_E",
			Args: []string{"-E", `a\nb`},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3 + R2.1: Combined -n -e.
		{
			Name: "echo_combined_n_e",
			Args: []string{"-n", "-e", `x\ty`},
			Env:  []string{"LC_ALL=C"},
		},
		// D3: Clustered flags -ne.
		{
			Name: "echo_clustered_ne",
			Args: []string{"-ne", `a\tb`},
			Env:  []string{"LC_ALL=C"},
		},
		// D3: Clustered flags -en.
		{
			Name: "echo_clustered_en",
			Args: []string{"-en", `x\ny`},
			Env:  []string{"LC_ALL=C"},
		},
		// D3: Clustered flags -neE (last E wins, escapes disabled).
		{
			Name: "echo_clustered_neE",
			Args: []string{"-neE", `a\tb`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.4: Last -e/-E wins; -E then -e means escapes enabled.
		{
			Name: "echo_last_flag_wins_e",
			Args: []string{"-E", "-e", `a\tb`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.4: Last -e/-E wins; -e then -E means escapes disabled.
		{
			Name: "echo_last_flag_wins_E",
			Args: []string{"-e", "-E", `a\tb`},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: Unrecognized flag treated as positional argument.
		{
			Name: "echo_unrecognized_flag_literal",
			Args: []string{"-z", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: After a non-flag arg, dash args are literal.
		{
			Name: "echo_flag_after_nonflag_literal",
			Args: []string{"hello", "-n", "world"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with \\ (escaped backslash).
		{
			Name: "echo_escape_backslash",
			Args: []string{"-e", `a\\b`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with \a (alert/BEL).
		{
			Name: "echo_escape_alert",
			Args: []string{"-e", `\a`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with \b (backspace).
		{
			Name: "echo_escape_backspace",
			Args: []string{"-e", `x\by`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with \e (escape character).
		{
			Name: "echo_escape_esc",
			Args: []string{"-e", `\e[31m`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with \f (form feed).
		{
			Name: "echo_escape_formfeed",
			Args: []string{"-e", `\f`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with \r (carriage return).
		{
			Name: "echo_escape_cr",
			Args: []string{"-e", `a\rb`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with \v (vertical tab).
		{
			Name: "echo_escape_vtab",
			Args: []string{"-e", `\v`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with \0NNN octal escape.
		{
			Name: "echo_escape_octal",
			Args: []string{"-e", `\0101`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with \xHH hex escape.
		{
			Name: "echo_escape_hex",
			Args: []string{"-e", `\x41`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: \c across arguments — stops all further output.
		{
			Name: "echo_escape_c_multiarg",
			Args: []string{"-e", `before\c`, "after"},
			Env:  []string{"LC_ALL=C"},
		},
		// Multiple arguments with spaces.
		{
			Name: "echo_multiple_args",
			Args: []string{"one", "two", "three"},
			Env:  []string{"LC_ALL=C"},
		},
		// Single argument.
		{
			Name: "echo_single_arg",
			Args: []string{"hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// Empty string argument.
		{
			Name: "echo_empty_string_arg",
			Args: []string{""},
			Env:  []string{"LC_ALL=C"},
		},
		// -n with no arguments.
		{
			Name: "echo_n_no_args",
			Args: []string{"-n"},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
