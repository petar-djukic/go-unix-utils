// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/echo against gecho.
// Implements: prd020-echo R4.1, R4.2, R4.3 (R1.1-R1.4, R2.1-R2.4, R3.1, R3.2, R3.3)
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gecho")
	if err != nil {
		t.Skipf("reference binary gecho not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.2: No arguments prints only a newline.
		{
			Name: "no arguments",
			Args: []string{},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: Single argument followed by newline.
		{
			Name: "single argument",
			Args: []string{"hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: Multiple arguments separated by spaces.
		{
			Name: "multiple arguments",
			Args: []string{"hello", "world"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: -n suppresses trailing newline.
		{
			Name: "suppress newline",
			Args: []string{"-n", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: -n with multiple arguments.
		{
			Name: "suppress newline multiple args",
			Args: []string{"-n", "hello", "world"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: Unrecognized flag treated as positional.
		{
			Name: "unrecognized flag as literal",
			Args: []string{"-z", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: Bare dash is a positional argument.
		{
			Name: "dash alone is literal",
			Args: []string{"-"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: Mixed valid/invalid flag chars treated as literal.
		{
			Name: "mixed valid and invalid flag chars",
			Args: []string{"-nz", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: Double dash is not special in echo; treated as literal.
		{
			Name: "double dash is literal",
			Args: []string{"--", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: Flag-like arg after positional is literal.
		{
			Name: "flag after positional is literal",
			Args: []string{"hello", "-n"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: Empty string argument.
		{
			Name: "empty string argument",
			Args: []string{""},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: Multiple empty strings produce spaces.
		{
			Name: "multiple empty strings",
			Args: []string{"", "", ""},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: -n with no further arguments.
		{
			Name: "suppress newline no args",
			Args: []string{"-n"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3, R1.4: Combined flag -nE recognized.
		{
			Name: "combined nE flag",
			Args: []string{"-nE", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with newline escape.
		{
			Name: "escape newline",
			Args: []string{"-e", `hello\nworld`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with tab escape.
		{
			Name: "escape tab",
			Args: []string{"-e", `a\tb`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with backslash escape.
		{
			Name: "escape backslash",
			Args: []string{"-e", `a\\b`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with alert (BEL).
		{
			Name: "escape alert",
			Args: []string{"-e", `\a`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with backspace.
		{
			Name: "escape backspace",
			Args: []string{"-e", `\b`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with ESC character.
		{
			Name: "escape esc",
			Args: []string{"-e", `\e`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with form feed.
		{
			Name: "escape form feed",
			Args: []string{"-e", `\f`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with carriage return.
		{
			Name: "escape carriage return",
			Args: []string{"-e", `\r`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with vertical tab.
		{
			Name: "escape vertical tab",
			Args: []string{"-e", `\v`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with octal escape \0101 = 'A'.
		{
			Name: "escape octal A",
			Args: []string{"-e", `\0101`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with octal escape \0 (null byte).
		{
			Name: "escape octal null",
			Args: []string{"-e", `\0`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with hex escape \x41 = 'A'.
		{
			Name: "escape hex A",
			Args: []string{"-e", `\x41`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -e with single hex digit \x9.
		{
			Name: "escape hex single digit",
			Args: []string{"-e", `\x9`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: \c suppresses further output and trailing newline.
		{
			Name: "escape c stops output",
			Args: []string{"-e", `stop\chere`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: \c with multiple arguments suppresses remaining args.
		{
			Name: "escape c multiple args",
			Args: []string{"-e", `first\c`, "second"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: -E disables escape interpretation.
		{
			Name: "E flag disables escapes",
			Args: []string{"-E", `hello\nworld`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.4: Last of -e and -E wins (e last).
		{
			Name: "e after E enables escapes",
			Args: []string{"-E", "-e", `hello\nworld`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.4: Last of -e and -E wins (E last).
		{
			Name: "E after e disables escapes",
			Args: []string{"-e", "-E", `hello\nworld`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.4: Combined -eE in single flag group (E last).
		{
			Name: "combined eE flag",
			Args: []string{"-eE", `hello\nworld`},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3 + R2.1: Combined -ne.
		{
			Name: "combined n and e",
			Args: []string{"-ne", `hello\nworld`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: Multiple escapes in one string.
		{
			Name: "multiple escapes",
			Args: []string{"-e", `\ta\nb\t`},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: Unknown escape passed through.
		{
			Name: "unknown escape literal",
			Args: []string{"-e", `\z`},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: Successful output exits 0.
		{
			Name:     "exit 0 on success",
			Args:     []string{"hello"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.1: Successful output with -n exits 0.
		{
			Name:     "exit 0 on success with n flag",
			Args:     []string{"-n", "hello"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.1: Successful output with -e exits 0.
		{
			Name:     "exit 0 on success with e flag",
			Args:     []string{"-e", `hello\nworld`},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.1: No arguments exits 0.
		{
			Name:     "exit 0 no arguments",
			Args:     []string{},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
