// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd020-echo R4.1, R4.2, R4.3.
// R4.1: compare Go echo against gecho byte-for-byte via RunDiffTests.
// R4.2: cover no args, single arg, multiple args, -n, -e escapes, -E,
//
//	combined -n -e, and arguments starting with '-'.
//
// R4.3: LC_ALL=C set by default in RunDiffTests.
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
		t.Skip("reference binary gecho not in PATH")
	}

	tests := []testutils.DiffTest{
		// R4.2: no arguments — outputs only a newline (R1.2).
		{Name: "no_args", Args: []string{}},

		// R4.2: single argument (R1.1).
		{Name: "single_arg", Args: []string{"hello"}},

		// R4.2: multiple arguments joined by spaces (R1.1).
		{Name: "multiple_args", Args: []string{"hello", "world", "foo"}},

		// R4.2: -n flag suppresses trailing newline (R1.3).
		{Name: "flag_n", Args: []string{"-n", "hello"}},

		// R4.2: -n with no arguments — no output at all.
		{Name: "flag_n_no_args", Args: []string{"-n"}},

		// R4.2: -E flag (default, escapes disabled) (R2.3).
		{Name: "flag_E", Args: []string{"-E", `hello\tworld`}},

		// R4.2: -e with tab escape (R2.1).
		{Name: "escape_tab", Args: []string{"-e", `a\tb`}},

		// R4.2: -e with newline escape (R2.1).
		{Name: "escape_newline", Args: []string{"-e", `a\nb`}},

		// R4.2: -e with backslash escape (R2.1).
		{Name: "escape_backslash", Args: []string{"-e", `a\\b`}},

		// R4.2: -e with alert/BEL (R2.1).
		{Name: "escape_alert", Args: []string{"-e", `\a`}},

		// R4.2: -e with backspace (R2.1).
		{Name: "escape_backspace", Args: []string{"-e", `\b`}},

		// R4.2: -e with escape character (R2.1).
		{Name: "escape_esc", Args: []string{"-e", `\e`}},

		// R4.2: -e with form feed (R2.1).
		{Name: "escape_formfeed", Args: []string{"-e", `\f`}},

		// R4.2: -e with carriage return (R2.1).
		{Name: "escape_cr", Args: []string{"-e", `\r`}},

		// R4.2: -e with vertical tab (R2.1).
		{Name: "escape_vtab", Args: []string{"-e", `\v`}},

		// R4.2: -e with \c stops output (R2.2).
		{Name: "escape_c_stop", Args: []string{"-e", `before\cafter`}},

		// R4.2: -e with octal \0NNN (R2.1).
		{Name: "escape_octal", Args: []string{"-e", `\0101`}},

		// R4.2: -e with hex \xHH (R2.1).
		{Name: "escape_hex", Args: []string{"-e", `\x41`}},

		// R4.2: combined -n -e (R1.3, R2.1).
		{Name: "combined_n_e", Args: []string{"-n", "-e", `a\tb`}},

		// R4.2: combined -ne in single flag group.
		{Name: "combined_ne_single", Args: []string{"-ne", `a\tb`}},

		// R4.2: -e then -E — last wins, escapes disabled (R2.4).
		{Name: "e_then_E", Args: []string{"-e", "-E", `a\tb`}},

		// R4.2: -E then -e — last wins, escapes enabled (R2.4).
		{Name: "E_then_e", Args: []string{"-E", "-e", `a\tb`}},

		// R4.2: arguments starting with '-' that are not valid flags (R1.4).
		{Name: "unrecognized_dash_arg", Args: []string{"-z", "hello"}},

		// R4.2: argument starting with '--' (R1.4).
		{Name: "double_dash_arg", Args: []string{"--", "hello"}},

		// Edge: empty string argument.
		{Name: "empty_string_arg", Args: []string{""}},

		// Edge: multiple -n flags.
		{Name: "multiple_n", Args: []string{"-n", "-n", "hello"}},

		// Edge: -e with \0 and no following octal digits.
		{Name: "escape_octal_zero_only", Args: []string{"-e", `\0`}},

		// Edge: -e with \x and no following hex digits.
		{Name: "escape_hex_no_digits", Args: []string{"-e", `\x`}},

		// Edge: -e with unknown escape — passed through literally.
		{Name: "escape_unknown", Args: []string{"-e", `\z`}},

		// Edge: -e with \c as only content in multiple args.
		{Name: "escape_c_multi_args", Args: []string{"-e", `\c`, "more"}},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
