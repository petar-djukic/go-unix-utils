// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/env.
// Tests cover srd039-env R3.1, R3.2, R3.3, R4.1, R4.2, R4.3.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
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

// sortNulLinesNormalizer sorts NUL-delimited entries to handle environment
// variable ordering differences when -0 is used.
func sortNulLinesNormalizer(b []byte) []byte {
	s := string(b)
	if s == "" {
		return b
	}
	// Split on NUL, sort, rejoin with NUL.
	parts := strings.Split(s, "\x00")
	// Remove trailing empty entry from final NUL.
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	sort.Strings(parts)
	return []byte(strings.Join(parts, "\x00") + "\x00")
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

	// Look up printenv for R4.2 command execution tests.
	printenvBin, _ := exec.LookPath("gprintenv")

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

		// R4.1: differential test comparing stdout and exit code for
		// environment dump with explicit env to verify byte-for-byte match.
		{
			Name:      "r4_env_dump_explicit",
			Args:      []string{},
			Env:       []string{"ALPHA=1", "BETA=2", "GAMMA=3"},
			Normalize: []testutils.NormalizeFunc{sortLinesNormalizer},
		},

		// R4.2: no-argument environment dump (inheriting current env).
		{
			Name:      "r4_no_arg_dump_inherited",
			Args:      []string{},
			Normalize: []testutils.NormalizeFunc{sortLinesNormalizer},
		},

		// R4.2: NAME=VALUE setting without command prints env with new var.
		{
			Name:      "r4_set_var_no_command",
			Args:      []string{"NEW_VAR=value123"},
			Env:       []string{"EXISTING=yes"},
			Normalize: []testutils.NormalizeFunc{sortLinesNormalizer},
		},

		// R4.2: multiple NAME=VALUE pairs without command.
		{
			Name:      "r4_multiple_vars_no_command",
			Args:      []string{"A=1", "B=2", "C=3"},
			Env:       []string{},
			Normalize: []testutils.NormalizeFunc{sortLinesNormalizer},
		},

		// R4.2: -u unsetting a variable from explicit env, no command.
		{
			Name: "r4_unset_no_command",
			Args: []string{"-u", "REMOVE_ME"},
			Env:  []string{"REMOVE_ME=gone", "KEEP_ME=stay"},
		},

		// R4.2: -u unsetting a variable that does not exist (no-op).
		{
			Name: "r4_unset_nonexistent",
			Args: []string{"-u", "DOES_NOT_EXIST_XYZ"},
			Env:  []string{"KEEP=yes"},
		},

		// R4.2: -i empty environment, no command.
		{
			Name: "r4_empty_env_no_command",
			Args: []string{"-i"},
		},

		// R4.2: -i with NAME=VALUE, no command.
		{
			Name:      "r4_empty_env_with_vars",
			Args:      []string{"-i", "X=10", "Y=20"},
			Normalize: []testutils.NormalizeFunc{sortLinesNormalizer},
		},

		// R4.2: -0 NUL-delimited output with explicit env.
		{
			Name:      "r4_null_delimited",
			Args:      []string{"-0"},
			Env:       []string{"P=1", "Q=2"},
			Normalize: []testutils.NormalizeFunc{sortNulLinesNormalizer},
		},

		// R4.2: -0 combined with -i and NAME=VALUE.
		{
			Name:      "r4_null_with_ignore",
			Args:      []string{"-i", "-0", "ONLY=this"},
			Normalize: []testutils.NormalizeFunc{sortNulLinesNormalizer},
		},

		// R4.2: COMMAND execution with modified env via /bin/sh.
		{
			Name: "r4_exec_with_modified_env",
			Args: []string{"-i", "TESTVAR=hello", "/bin/sh", "-c", "echo $TESTVAR"},
		},

		// R4.2: COMMAND execution inheriting env with NAME=VALUE override.
		{
			Name: "r4_exec_override_env",
			Args: []string{"MY_OVERRIDE=new", "/bin/sh", "-c", "echo $MY_OVERRIDE"},
			Env:  []string{"MY_OVERRIDE=old"},
		},

		// R4.2: COMMAND execution with -u removing a variable before command.
		{
			Name: "r4_exec_with_unset",
			Args: []string{"-u", "DROP", "KEEP=yes",
				"/bin/sh", "-c", "echo KEEP=$KEEP DROP=$DROP"},
			Env: []string{"DROP=removeMe", "KEEP=old"},
		},

		// R4.2: exit code passthrough — exit 0.
		{
			Name:     "r4_exit_code_zero",
			Args:     []string{"/bin/sh", "-c", "exit 0"},
			ExitCode: 0,
		},

		// R4.2: exit code passthrough — exit 1.
		{
			Name:     "r4_exit_code_one",
			Args:     []string{"/bin/sh", "-c", "exit 1"},
			ExitCode: 1,
		},

		// R4.2: exit code passthrough — exit 2.
		{
			Name:     "r4_exit_code_two",
			Args:     []string{"/bin/sh", "-c", "exit 2"},
			ExitCode: 2,
		},

		// R4.3: invalid short option exits 125 with error on stderr.
		{
			Name:      "r4_invalid_short_option",
			Args:      []string{"-Z"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer, discardOutput},
		},

		// R4.3: invalid long option exits 125 with error on stderr.
		{
			Name:      "r4_invalid_long_option",
			Args:      []string{"--nonexistent-flag"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R4.3: command not found exits 127 with error on stderr.
		{
			Name:      "r4_missing_command",
			Args:      []string{"this_command_does_not_exist_abc123"},
			ExitCode:  127,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	// R4.2: COMMAND execution using printenv to verify env variable is
	// passed through to the child process (requires gprintenv).
	if printenvBin != "" {
		tests = append(tests, testutils.DiffTest{
			Name: "r4_exec_printenv_set_var",
			Args: []string{"-i", "TESTKEY=testval", "printenv", "TESTKEY"},
		})
	}

	// R4.3: non-executable file exits 126.
	tmpDir := t.TempDir()
	nonExecFile := filepath.Join(tmpDir, "not_executable")
	if err := os.WriteFile(nonExecFile, []byte("#!/bin/sh\n"), 0o644); err == nil {
		tests = append(tests, testutils.DiffTest{
			Name:      "r4_not_executable",
			Args:      []string{nonExecFile},
			ExitCode:  126,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		})
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
