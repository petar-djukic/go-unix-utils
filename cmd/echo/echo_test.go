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

		// R2.1: -e escape sequences.
		{
			Name: "escape_backslash",
			Args: []string{"-e", `a\\b`},
		},
		{
			Name: "escape_alert",
			Args: []string{"-e", `\a`},
		},
		{
			Name: "escape_backspace",
			Args: []string{"-e", `\b`},
		},
		{
			Name: "escape_escape_char",
			Args: []string{"-e", `\e`},
		},
		{
			Name: "escape_form_feed",
			Args: []string{"-e", `\f`},
		},
		{
			Name: "escape_newline",
			Args: []string{"-e", `a\nb`},
		},
		{
			Name: "escape_carriage_return",
			Args: []string{"-e", `a\rb`},
		},
		{
			Name: "escape_tab",
			Args: []string{"-e", `a\tb`},
		},
		{
			Name: "escape_vertical_tab",
			Args: []string{"-e", `\v`},
		},
		{
			Name: "escape_octal_zero",
			Args: []string{"-e", `\0101`},
		},
		{
			Name: "escape_octal_one_digit",
			Args: []string{"-e", `\01`},
		},
		{
			Name: "escape_octal_no_digits",
			Args: []string{"-e", `\0`},
		},
		{
			Name: "escape_hex",
			Args: []string{"-e", `\x41`},
		},
		{
			Name: "escape_hex_single_digit",
			Args: []string{"-e", `\x9`},
		},
		{
			Name: "escape_hex_lowercase",
			Args: []string{"-e", `\x6a`},
		},
		{
			Name: "escape_multiple_sequences",
			Args: []string{"-e", `\t\n\t`},
		},
		{
			Name: "escape_unknown_backslash",
			Args: []string{"-e", `\z`},
		},

		// R2.2: \c terminates output.
		{
			Name: "escape_c_suppresses_rest",
			Args: []string{"-e", `before\cafter`},
		},
		{
			Name: "escape_c_suppresses_further_args",
			Args: []string{"-e", `hello\c`, "world"},
		},
		{
			Name: "escape_c_alone",
			Args: []string{"-e", `\c`},
		},

		// R2.3: -E disables escape interpretation (default).
		{
			Name: "flag_E_no_escape",
			Args: []string{"-E", `hello\tworld`},
		},
		{
			Name: "default_no_escape",
			Args: []string{`hello\tworld`},
		},

		// R2.4: Last of -e / -E wins.
		{
			Name: "e_then_E_disables",
			Args: []string{"-e", "-E", `hello\tworld`},
		},
		{
			Name: "E_then_e_enables",
			Args: []string{"-E", "-e", `hello\tworld`},
		},
		{
			Name: "combined_eE_last_E_wins",
			Args: []string{"-eE", `hello\tworld`},
		},
		{
			Name: "combined_Ee_last_e_wins",
			Args: []string{"-Ee", `hello\tworld`},
		},
		{
			Name: "n_and_e_combined",
			Args: []string{"-ne", `a\tb`},
		},

		// R3.1: Exit 0 on successful output.
		{
			Name:     "exit_zero_on_success",
			Args:     []string{"hello"},
			ExitCode: 0,
		},
		{
			Name:     "exit_zero_no_args",
			Args:     []string{},
			ExitCode: 0,
		},
		{
			Name:     "exit_zero_with_escapes",
			Args:     []string{"-e", `a\tb`},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
