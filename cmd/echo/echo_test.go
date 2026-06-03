// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

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
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		// R1.2: no arguments → newline only
		{Name: "no-args", Env: []string{"LC_ALL=C"}},
		// R1.1: single argument
		{Name: "single-arg", Args: []string{"hello"}, Env: []string{"LC_ALL=C"}},
		// R1.1: multiple arguments joined by spaces
		{Name: "multiple-args", Args: []string{"hello", "world"}, Env: []string{"LC_ALL=C"}},
		// R1.1: three arguments
		{Name: "three-args", Args: []string{"a", "b", "c"}, Env: []string{"LC_ALL=C"}},
		// R1.3: -n suppresses trailing newline
		{Name: "n-flag", Args: []string{"-n", "hello"}, Env: []string{"LC_ALL=C"}},
		// R1.3: -n with no arguments
		{Name: "n-no-args", Args: []string{"-n"}, Env: []string{"LC_ALL=C"}},
		// R1.3: -n with multiple arguments
		{Name: "n-multiple", Args: []string{"-n", "hello", "world"}, Env: []string{"LC_ALL=C"}},
		// R1.4: unrecognized flag treated as literal
		{Name: "unrecognized-flag", Args: []string{"-z", "hello"}, Env: []string{"LC_ALL=C"}},
		// R1.4: bare dash is literal
		{Name: "bare-dash", Args: []string{"-"}, Env: []string{"LC_ALL=C"}},
		// R1.4: double dash is literal (echo has no -- support)
		{Name: "double-dash", Args: []string{"--", "hello"}, Env: []string{"LC_ALL=C"}},
		// R1.4: -e is a recognized flag, not literal
		{Name: "e-flag", Args: []string{"-e", "hello"}, Env: []string{"LC_ALL=C"}},
		// R1.4: -E is a recognized flag, not literal
		{Name: "E-flag", Args: []string{"-E", "hello"}, Env: []string{"LC_ALL=C"}},
		// R1.4: combined recognized flags
		{Name: "nE-combined", Args: []string{"-nE", "hello"}, Env: []string{"LC_ALL=C"}},
		{Name: "ne-combined", Args: []string{"-ne", "hello"}, Env: []string{"LC_ALL=C"}},
		{Name: "nEe-combined", Args: []string{"-nEe", "hello"}, Env: []string{"LC_ALL=C"}},
		// R1.4: mixed valid/invalid chars in flag → literal
		{Name: "mixed-flag-invalid", Args: []string{"-nz", "hello"}, Env: []string{"LC_ALL=C"}},
		{Name: "mixed-flag-invalid2", Args: []string{"-en!", "hello"}, Env: []string{"LC_ALL=C"}},
		// R1.4: flag-like args after non-flag are literal
		{Name: "flag-after-arg", Args: []string{"hello", "-n"}, Env: []string{"LC_ALL=C"}},
		// R1.4: multiple flag args before operands
		{Name: "multiple-flag-args", Args: []string{"-n", "-e", "hello"}, Env: []string{"LC_ALL=C"}},
		// --help and --version are non-goals per SRD (gecho handles them specially)
		// R1.1: empty string argument
		{Name: "empty-string", Args: []string{""}, Env: []string{"LC_ALL=C"}},
		// R1.1: multiple empty strings
		{Name: "multiple-empty", Args: []string{"", ""}, Env: []string{"LC_ALL=C"}},
		// R1.1: argument with spaces
		{Name: "arg-with-spaces", Args: []string{"hello world"}, Env: []string{"LC_ALL=C"}},
		// escape sequences with -e
		{Name: "e-tab", Args: []string{"-e", `a\tb`}, Env: []string{"LC_ALL=C"}},
		{Name: "e-newline", Args: []string{"-e", `a\nb`}, Env: []string{"LC_ALL=C"}},
		{Name: "e-backslash", Args: []string{"-e", `a\\b`}, Env: []string{"LC_ALL=C"}},
		{Name: "e-alert", Args: []string{"-e", `\a`}, Env: []string{"LC_ALL=C"}},
		{Name: "e-backspace", Args: []string{"-e", `\b`}, Env: []string{"LC_ALL=C"}},
		{Name: "e-escape", Args: []string{"-e", `\e`}, Env: []string{"LC_ALL=C"}},
		{Name: "e-formfeed", Args: []string{"-e", `\f`}, Env: []string{"LC_ALL=C"}},
		{Name: "e-cr", Args: []string{"-e", `\r`}, Env: []string{"LC_ALL=C"}},
		{Name: "e-vtab", Args: []string{"-e", `\v`}, Env: []string{"LC_ALL=C"}},
		{Name: "e-suppress-c", Args: []string{"-e", `before\cafter`}, Env: []string{"LC_ALL=C"}},
		{Name: "e-octal", Args: []string{"-e", `\0101`}, Env: []string{"LC_ALL=C"}},
		{Name: "e-hex", Args: []string{"-e", `\x41`}, Env: []string{"LC_ALL=C"}},
		// -E disables escapes (default behavior)
		{Name: "E-no-escape", Args: []string{"-E", `a\tb`}, Env: []string{"LC_ALL=C"}},
		// -e then -E: last wins
		{Name: "eE-last-wins", Args: []string{"-e", "-E", `a\tb`}, Env: []string{"LC_ALL=C"}},
		// -E then -e: last wins
		{Name: "Ee-last-wins", Args: []string{"-E", "-e", `a\tb`}, Env: []string{"LC_ALL=C"}},
		// combined -n -e
		{Name: "ne-suppress-newline-and-escape", Args: []string{"-ne", `a\tb`}, Env: []string{"LC_ALL=C"}},
		// R2.1: octal edge cases
		{Name: "e-octal-zero", Args: []string{"-e", `\0`}, Env: []string{"LC_ALL=C"}},
		{Name: "e-octal-one-digit", Args: []string{"-e", `\07`}, Env: []string{"LC_ALL=C"}},
		{Name: "e-octal-two-digits", Args: []string{"-e", `\077`}, Env: []string{"LC_ALL=C"}},
		{Name: "e-octal-max", Args: []string{"-e", `\0377`}, Env: []string{"LC_ALL=C"}},
		// R2.1: hex edge cases
		{Name: "e-hex-one-digit", Args: []string{"-e", `\xA`}, Env: []string{"LC_ALL=C"}},
		{Name: "e-hex-lower", Args: []string{"-e", `\x6f`}, Env: []string{"LC_ALL=C"}},
		{Name: "e-hex-no-digits", Args: []string{"-e", `\xZZ`}, Env: []string{"LC_ALL=C"}},
		// R2.1: unknown escape passes through literally
		{Name: "e-unknown-escape", Args: []string{"-e", `\q`}, Env: []string{"LC_ALL=C"}},
		// R2.1: multiple escapes in one string
		{Name: "e-multiple-escapes", Args: []string{"-e", `\t\n\a`}, Env: []string{"LC_ALL=C"}},
		// R2.2: \c suppresses further arguments too
		{Name: "e-c-multi-arg", Args: []string{"-e", `before\c`, "after"}, Env: []string{"LC_ALL=C"}},
		// R2.2: \c at start
		{Name: "e-c-at-start", Args: []string{"-e", `\crest`}, Env: []string{"LC_ALL=C"}},
		// R2.3: -E with backslash sequences written literally
		{Name: "E-literal-backslash-n", Args: []string{"-E", `a\nb`}, Env: []string{"LC_ALL=C"}},
		// R2.4: -eE combined in single flag (last char wins)
		{Name: "eE-combined-flag", Args: []string{"-eE", `a\tb`}, Env: []string{"LC_ALL=C"}},
		// R2.4: -Ee combined in single flag (last char wins)
		{Name: "Ee-combined-flag", Args: []string{"-Ee", `a\tb`}, Env: []string{"LC_ALL=C"}},
		// R2.4: multiple separate flags, last wins
		{Name: "e-E-e-last-wins", Args: []string{"-e", "-E", "-e", `a\tb`}, Env: []string{"LC_ALL=C"}},
		// combined -n with -E then -e
		{Name: "n-E-e-combined", Args: []string{"-n", "-E", "-e", `a\tb`}, Env: []string{"LC_ALL=C"}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
