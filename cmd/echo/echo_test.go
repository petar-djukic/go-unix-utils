// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd020-echo R1.1–R1.4, R2.1–R2.4, R3.1–R3.3, R4.1–R4.3.
// R4.1: differential tests compare Go echo output against gecho via RunDiffTests.
// R4.2: tests cover no args, single/multiple args, -n, -e escapes, -E, combined -n -e, dash args.
// R4.3: LC_ALL=C set by default in testutils.RunDiffTests harness.
package main

import (
	"os"
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

		// R2.1: -e backslash (\\)
		{
			Name: "escape_backslash",
			Args: []string{"-e", `a\\b`},
		},
		// R2.1: -e alert (\a)
		{
			Name: "escape_alert",
			Args: []string{"-e", `\a`},
		},
		// R2.1: -e backspace (\b)
		{
			Name: "escape_backspace",
			Args: []string{"-e", `\b`},
		},
		// R2.1: -e escape (\e)
		{
			Name: "escape_esc",
			Args: []string{"-e", `\e`},
		},
		// R2.1: -e form feed (\f)
		{
			Name: "escape_formfeed",
			Args: []string{"-e", `\f`},
		},
		// R2.1: -e newline (\n)
		{
			Name: "escape_newline",
			Args: []string{"-e", `\n`},
		},
		// R2.1: -e carriage return (\r)
		{
			Name: "escape_cr",
			Args: []string{"-e", `\r`},
		},
		// R2.1: -e horizontal tab (\t)
		{
			Name: "escape_tab",
			Args: []string{"-e", `\t`},
		},
		// R2.1: -e vertical tab (\v)
		{
			Name: "escape_vtab",
			Args: []string{"-e", `\v`},
		},
		// R2.1: -e octal (\0NNN)
		{
			Name: "escape_octal_041",
			Args: []string{"-e", `\041`},
		},
		// R2.1: -e octal zero (\0)
		{
			Name: "escape_octal_zero",
			Args: []string{"-e", `a\0b`},
		},
		// R2.1: -e octal three digits (\0101 = 'A')
		{
			Name: "escape_octal_101",
			Args: []string{"-e", `\0101`},
		},
		// R2.1: -e hex (\xHH)
		{
			Name: "escape_hex_41",
			Args: []string{"-e", `\x41`},
		},
		// R2.1: -e hex single digit
		{
			Name: "escape_hex_single",
			Args: []string{"-e", `\x9`},
		},
		// R2.1: -e hex uppercase
		{
			Name: "escape_hex_upper",
			Args: []string{"-e", `\x4F`},
		},
		// R2.2: \c terminates output
		{
			Name: "escape_c_truncate",
			Args: []string{"-e", `before\cafter`},
		},
		// R2.2: \c suppresses trailing newline
		{
			Name: "escape_c_no_newline",
			Args: []string{"-e", `hello\c`},
		},
		// R2.2: \c with multiple arguments suppresses remaining
		{
			Name: "escape_c_multi_args",
			Args: []string{"-e", `first\c`, "second"},
		},
		// R2.3: -E disables escapes (literal backslash sequences)
		{
			Name: "flag_E_literal",
			Args: []string{"-E", `hello\nworld`},
		},
		// R2.3: default behavior (no -e) treats escapes as literal
		{
			Name: "default_no_escapes",
			Args: []string{`hello\nworld`},
		},
		// R2.4: -e then -E, last wins (escapes disabled)
		{
			Name: "flag_e_then_E",
			Args: []string{"-e", "-E", `hello\nworld`},
		},
		// R2.4: -E then -e, last wins (escapes enabled)
		{
			Name: "flag_E_then_e",
			Args: []string{"-E", "-e", `hello\nworld`},
		},
		// R2.4: combined -eE in single arg, last char wins
		{
			Name: "flag_eE_combined",
			Args: []string{"-eE", `hello\nworld`},
		},
		// R2.4: combined -Ee in single arg, last char wins
		{
			Name: "flag_Ee_combined",
			Args: []string{"-Ee", `hello\nworld`},
		},
		// R2.1+R1.3: -ne combined flags
		{
			Name: "flag_ne_combined",
			Args: []string{"-ne", `hello\nworld`},
		},
		// R2.1: -e with multiple escape sequences
		{
			Name: "escape_multiple_sequences",
			Args: []string{"-e", `\t\n\t`},
		},
		// R2.1: -e with unrecognized escape (literal passthrough)
		{
			Name: "escape_unrecognized",
			Args: []string{"-e", `\q`},
		},
		// R2.1: -e trailing backslash
		{
			Name: "escape_trailing_backslash",
			Args: []string{"-e", `hello\`},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestExitCodeSuccess verifies R3.1: exit 0 on successful output.
func TestExitCodeSuccess(t *testing.T) {
	t.Parallel()
	exitCode := run([]string{"hello"}, os.Stdout)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
}

// TestExitCodeWriteError verifies R3.2: exit 1 when stdout write fails.
func TestExitCodeWriteError(t *testing.T) {
	t.Parallel()
	f := createClosedFile(t)
	exitCode := run([]string{"hello"}, f)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1 on write error, got %d", exitCode)
	}
}

// createClosedFile returns an *os.File whose write end is broken,
// causing any write to fail.
func createClosedFile(t *testing.T) *os.File {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	r.Close()
	w.Close()
	return w
}

// TestSIGPIPE verifies R3.3: echo terminates cleanly on broken pipe.
// The SIGPIPE handler (exit 0) races with the write error path (exit 1),
// so we accept either exit code — the key property is no crash or hang.
func TestSIGPIPE(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	r.Close() // close read end so write triggers SIGPIPE
	cmd := exec.Command(goBin, "hello")
	cmd.Stdout = w
	err = cmd.Run()
	w.Close() // best-effort cleanup
	if err == nil {
		return // exit 0: SIGPIPE handler won the race
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("unexpected error type: %v", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit 0 or 1 on SIGPIPE, got %d", exitErr.ExitCode())
	}
}
