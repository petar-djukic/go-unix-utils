// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestSIGPIPE verifies that cmd/echo handles a closed stdout gracefully (R3.2, R3.3).
// When stdout is closed, the binary may receive SIGPIPE (exit 0 via handler) or
// detect a write error (exit 1 per R3.2). Both are correct; the key invariant is
// that the process does not crash or hang.
func TestSIGPIPE(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "hello")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Close the read end immediately to trigger SIGPIPE or write error.
	stdout.Close()

	err = cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			// R3.2: exit 1 on write error; R3.3: exit 0 on SIGPIPE.
			if code != 0 && code != 1 {
				t.Errorf("expected exit 0 or 1 on closed stdout, got %d", code)
			}
		}
	}
}

// TestDiff verifies cmd/echo against the GNU reference binary gecho.
// Implements prd020-echo R3.1-R3.3, R4.1-R4.3.
// R4.1: Differential tests compare Go echo output against gecho byte-for-byte.
// R4.2: Tests cover no arguments, single/multiple arguments, -n, -e escapes, -E, combined flags, dash args.
// R4.3: All test cases use LC_ALL=C to eliminate locale-dependent divergence.
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
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "multiple_arguments",
			Args: []string{"hello", "world"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "three_arguments",
			Args: []string{"a", "b", "c"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: No arguments produces only a newline.
		{
			Name: "no_arguments",
			Args: []string{},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: -n suppresses trailing newline.
		{
			Name: "flag_n_single_arg",
			Args: []string{"-n", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "flag_n_multiple_args",
			Args: []string{"-n", "hello", "world"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "flag_n_no_args",
			Args: []string{"-n"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: Unrecognized flags passed as literal text.
		{
			Name: "unrecognized_flag_literal",
			Args: []string{"-x", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "double_dash_literal",
			Args: []string{"--", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "dash_only_literal",
			Args: []string{"-"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "mixed_valid_invalid_flag_chars",
			Args: []string{"-nz", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: Combined recognized flags.
		{
			Name: "combined_nE_flag",
			Args: []string{"-nE", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "combined_ne_flag",
			Args: []string{"-ne", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: -e and -E are recognized flags (not literal).
		{
			Name: "flag_e_alone",
			Args: []string{"-e", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "flag_E_alone",
			Args: []string{"-E", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: Multiple separate flag args.
		{
			Name: "separate_n_e_flags",
			Args: []string{"-n", "-e", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: Arguments with special characters passed literally.
		{
			Name: "argument_with_spaces",
			Args: []string{"hello world"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "empty_string_argument",
			Args: []string{""},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "multiple_empty_arguments",
			Args: []string{"", ""},
			Env:  []string{"LC_ALL=C"},
		},

		// R2.1: -e escape sequences.
		{
			Name: "escape_backslash",
			Args: []string{"-e", `a\\b`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape_alert",
			Args: []string{"-e", `\a`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape_backspace",
			Args: []string{"-e", `\b`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape_escape_char",
			Args: []string{"-e", `\e`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape_form_feed",
			Args: []string{"-e", `\f`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape_newline",
			Args: []string{"-e", `a\nb`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape_carriage_return",
			Args: []string{"-e", `a\rb`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape_tab",
			Args: []string{"-e", `a\tb`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape_vertical_tab",
			Args: []string{"-e", `\v`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape_octal_zero",
			Args: []string{"-e", `\0101`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape_octal_one_digit",
			Args: []string{"-e", `\01`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape_octal_no_digits",
			Args: []string{"-e", `\0`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape_hex",
			Args: []string{"-e", `\x41`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape_hex_single_digit",
			Args: []string{"-e", `\x9`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape_hex_lowercase",
			Args: []string{"-e", `\x6a`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape_multiple_sequences",
			Args: []string{"-e", `\t\n\t`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape_unknown_backslash",
			Args: []string{"-e", `\z`},
			Env:  []string{"LC_ALL=C"},
		},

		// R2.2: \c terminates output.
		{
			Name: "escape_c_suppresses_rest",
			Args: []string{"-e", `before\cafter`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape_c_suppresses_further_args",
			Args: []string{"-e", `hello\c`, "world"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape_c_alone",
			Args: []string{"-e", `\c`},
			Env:  []string{"LC_ALL=C"},
		},

		// R2.3: -E disables escape interpretation (default).
		{
			Name: "flag_E_no_escape",
			Args: []string{"-E", `hello\tworld`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "default_no_escape",
			Args: []string{`hello\tworld`},
			Env:  []string{"LC_ALL=C"},
		},

		// R2.4: Last of -e / -E wins.
		{
			Name: "e_then_E_disables",
			Args: []string{"-e", "-E", `hello\tworld`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "E_then_e_enables",
			Args: []string{"-E", "-e", `hello\tworld`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "combined_eE_last_E_wins",
			Args: []string{"-eE", `hello\tworld`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "combined_Ee_last_e_wins",
			Args: []string{"-Ee", `hello\tworld`},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "n_and_e_combined",
			Args: []string{"-ne", `a\tb`},
			Env:  []string{"LC_ALL=C"},
		},

		// R3.1: Exit 0 on successful output.
		{
			Name:     "exit_zero_on_success",
			Args:     []string{"hello"},
			ExitCode: 0,
			Env:      []string{"LC_ALL=C"},
		},
		{
			Name:     "exit_zero_no_args",
			Args:     []string{},
			ExitCode: 0,
			Env:      []string{"LC_ALL=C"},
		},
		{
			Name:     "exit_zero_with_escapes",
			Args:     []string{"-e", `a\tb`},
			ExitCode: 0,
			Env:      []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
