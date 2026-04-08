// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/env.
// Tests cover srd039-env R3.1, R3.2, R3.3.
package main

import (
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgRe matches the program name/path prefix before a colon at line start.
var stderrProgRe = regexp.MustCompile(`(?m)^[^\s:]+:`)

// stderrTryRe matches the quoted program reference in Try hint lines.
var stderrTryRe = regexp.MustCompile(`'[^']*--help'`)

// stderrNormalizer normalizes program name differences in error messages.
// Replaces binary paths with "PROG" so error message structure can be
// compared between Go and GNU binaries.
func stderrNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	b = stderrProgRe.ReplaceAll(b, []byte("PROG:"))
	b = stderrTryRe.ReplaceAll(b, []byte("'PROG --help'"))
	return b
}

// sortLinesNormalizer sorts output lines to handle environment variable
// ordering differences between Go and GNU implementations.
func sortLinesNormalizer(b []byte) []byte {
	s := strings.TrimSuffix(string(b), "\n")
	if s == "" {
		return b
	}
	lines := strings.Split(s, "\n")
	sort.Strings(lines)
	return []byte(strings.Join(lines, "\n") + "\n")
}

// discardOutput normalizes by discarding all output, used when output
// content differs by design (--version, --help) and only exit code
// comparison is meaningful.
func discardOutput(b []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("genv")
	if err != nil {
		t.Skipf("reference binary genv not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R3.1: no arguments prints current environment.
		{
			Name: "no_args",
			Args: []string{},
		},

		// R3.1: no arguments with explicit env for deterministic output.
		{
			Name: "no_args_explicit_env",
			Args: []string{},
			Env:  []string{"FOO=bar", "BAZ=qux"},
		},

		// R3.1: -0 flag terminates each line with NUL instead of newline.
		{
			Name: "null_output",
			Args: []string{"-0"},
			Env:  []string{"A=1", "B=2"},
		},

		// R3.1: --null long form of -0.
		{
			Name: "null_long_flag",
			Args: []string{"--null"},
			Env:  []string{"X=hello"},
		},

		// R3.1: -i with -0 combined.
		{
			Name: "ignore_env_null",
			Args: []string{"-i", "-0", "X=1"},
		},

		// R3.2: -i starts with empty environment.
		{
			Name: "ignore_env_empty",
			Args: []string{"-i"},
		},

		// R3.2: -i with NAME=VALUE pairs.
		{
			Name:      "ignore_env_with_vars",
			Args:      []string{"-i", "FOO=hello", "BAR=world"},
			Normalize: []testutils.NormalizeFunc{sortLinesNormalizer},
		},

		// R3.2: --ignore-environment long form.
		{
			Name: "ignore_env_long_flag",
			Args: []string{"--ignore-environment", "A=1"},
		},

		// R3.2: bare dash implies -i.
		{
			Name: "bare_dash",
			Args: []string{"-", "FOO=bar"},
		},

		// R3.2: -u removes a variable from the environment.
		{
			Name: "unset_var",
			Args: []string{"-u", "FOO"},
			Env:  []string{"FOO=bar", "BAZ=qux"},
		},

		// R3.2: --unset=NAME long form.
		{
			Name: "unset_long_form",
			Args: []string{"--unset=FOO"},
			Env:  []string{"FOO=bar", "BAZ=qux"},
		},

		// R3.2: multiple -u flags.
		{
			Name: "unset_multiple",
			Args: []string{"-u", "A", "-u", "B"},
			Env:  []string{"A=1", "B=2", "C=3"},
		},

		// R3.2: NAME=VALUE overrides existing variable.
		{
			Name: "override_var",
			Args: []string{"FOO=new"},
			Env:  []string{"FOO=old", "BAZ=qux"},
		},

		// R3.2: command execution with /bin/echo.
		{
			Name: "exec_echo",
			Args: []string{"/bin/echo", "hello"},
		},

		// R3.2: command execution with modified environment.
		{
			Name: "exec_with_set_var",
			Args: []string{"-i", "MYVAR=test", "/bin/sh", "-c", "echo $MYVAR"},
		},

		// R3.2: exit code passthrough from command.
		{
			Name:     "exit_code_zero",
			Args:     []string{"/bin/sh", "-c", "exit 0"},
			ExitCode: 0,
		},

		// R3.2: non-zero exit code passthrough.
		{
			Name:     "exit_code_passthrough",
			Args:     []string{"/bin/sh", "-c", "exit 42"},
			ExitCode: 42,
		},

		// R3.2: --help exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R3.2: --version exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R3.3: invalid option exits 125.
		{
			Name:      "invalid_option",
			Args:      []string{"--bogus"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R3.3: command not found exits 127.
		{
			Name:      "command_not_found",
			Args:      []string{"nonexistent_command_xyz_12345"},
			ExitCode:  127,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
