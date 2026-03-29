// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/env against genv (GNU coreutils).
//
// Covers prd039-env R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R3.1, R3.2, R3.3,
// R4.1, R4.2, R4.3.
package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// sortLines sorts output lines for deterministic comparison.
func sortLines(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	s := strings.TrimSuffix(string(data), "\n")
	lines := strings.Split(s, "\n")
	sort.Strings(lines)
	return []byte(strings.Join(lines, "\n") + "\n")
}

// sortNulLines sorts NUL-delimited output for deterministic comparison.
func sortNulLines(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	s := strings.TrimSuffix(string(data), "\x00")
	lines := strings.Split(s, "\x00")
	sort.Strings(lines)
	return []byte(strings.Join(lines, "\x00") + "\x00")
}

// discardAll blanks output for exit-code-only comparison.
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("genv")
	if err != nil {
		t.Skip("reference binary genv not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1: no arguments prints all environment variables
		{
			Name:      "R1.1_print_all",
			Args:      []string{},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R1.2: execute a command
		{
			Name:     "R1.2_run_command",
			Args:     []string{"printenv", "HOME"},
			ExitCode: 0,
		},
		// R1.2: NAME=VALUE + command
		{
			Name:     "R1.2_set_and_run",
			Args:     []string{"MY_TEST_VAR=hello", "printenv", "MY_TEST_VAR"},
			ExitCode: 0,
		},
		// R1.3: command not found exits 127
		{
			Name:      "R1.3_not_found",
			Args:      []string{"nonexistent_command_xyz_42"},
			ExitCode:  127,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R2.1: -i starts with empty environment
		{
			Name:     "R2.1_ignore_env",
			Args:     []string{"-i", "MY_VAR=only", "env"},
			ExitCode: 0,
		},
		// R2.1: --ignore-environment long form
		{
			Name:     "R2.1_ignore_env_long",
			Args:     []string{"--ignore-environment", "SINGLE=val", "env"},
			ExitCode: 0,
		},
		// R2.1: -i with no command prints empty output
		{
			Name:     "R2.1_ignore_no_cmd",
			Args:     []string{"-i"},
			ExitCode: 0,
		},
		// R2.2: -u removes a variable from the environment
		{
			Name:     "R2.2_unset_short",
			Args:     []string{"-u", "HOME", "printenv", "HOME"},
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R2.2: --unset=NAME long form
		{
			Name:     "R2.2_unset_long",
			Args:     []string{"--unset=HOME", "printenv", "HOME"},
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R2.2: multiple -u flags unset from inherited environment
		{
			Name:      "R2.2_unset_multiple",
			Args:      []string{"-u", "HOME", "-u", "USER", "env"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R2.3: NAME=VALUE pairs set variables, first non-= arg is COMMAND
		{
			Name:     "R2.3_set_vars",
			Args:     []string{"FOO=bar", "BAZ=qux", "printenv", "FOO"},
			ExitCode: 0,
		},
		// R2.3: NAME=VALUE with empty value
		{
			Name:     "R2.3_empty_value",
			Args:     []string{"EMPTY_VAR=", "printenv", "EMPTY_VAR"},
			ExitCode: 0,
		},
		// R2.3: NAME=VALUE with = in value
		{
			Name:     "R2.3_equals_in_value",
			Args:     []string{"EQ_VAR=a=b=c", "printenv", "EQ_VAR"},
			ExitCode: 0,
		},
		// R3.1: -0 uses NUL delimiters for environment dump
		{
			Name:      "R3.1_null_delim",
			Args:      []string{"-i", "-0", "FOO=bar", "BAZ=qux"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{sortNulLines},
		},
		// R3.1: --null long form
		{
			Name:      "R3.1_null_long",
			Args:      []string{"-i", "--null", "X=1"},
			ExitCode:  0,
		},
		// R3.2: exit code passthrough from command
		{
			Name:     "R3.2_exit_passthrough_success",
			Args:     []string{"true"},
			ExitCode: 0,
		},
		// R3.2: exit code passthrough for failing command
		{
			Name:     "R3.2_exit_passthrough_failure",
			Args:     []string{"false"},
			ExitCode: 1,
		},
		// R3.3: invalid long option exits 125
		{
			Name:      "R3.3_invalid_long_option",
			Args:      []string{"--foobar"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R3.3: invalid short option exits 125
		{
			Name:      "R3.3_invalid_short_option",
			Args:      []string{"-z"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.3: -u missing argument exits 125
		{
			Name:      "R4.3_u_missing_arg",
			Args:      []string{"-u"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestExitCode126 verifies that a non-executable file produces exit code 126.
// R1.3: COMMAND found but cannot be executed must exit 126.
func TestExitCode126(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	notExec := filepath.Join(tmpDir, "notexec")
	if err := os.WriteFile(notExec, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("creating non-executable file: %v", err)
	}

	cmd := exec.Command(goBin, notExec)
	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %v", err)
	}
	if exitErr.ExitCode() != 126 {
		t.Errorf("exit code = %d, want 126", exitErr.ExitCode())
	}
}
