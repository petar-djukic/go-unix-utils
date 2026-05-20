// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("genv")
	if err != nil {
		t.Skip("reference binary genv not found")
	}

	envBin := findEnvBin(t)

	tests := []testutils.DiffTest{
		// R1.1: no-argument environment dump
		{
			Name:      "no_args_env_dump",
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R1.2: execute command with inherited environment
		{
			Name: "exec_echo",
			Args: []string{"echo", "hello"},
		},
		// R1.2: command with arguments
		{
			Name: "exec_echo_multiple_args",
			Args: []string{"echo", "foo", "bar", "baz"},
		},
		// R1.2 + R2.3: NAME=VALUE sets variable in child environment
		{
			Name:      "set_variable",
			Args:      []string{"FOO=bar", envBin},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R1.2 + R2.3: override existing variable
		{
			Name:      "override_variable",
			Args:      []string{"HOME=/tmp/test", envBin},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R1.2 + R2.3: multiple NAME=VALUE pairs
		{
			Name:      "multiple_vars",
			Args:      []string{"A=1", "B=2", "C=3", envBin},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R2.1: -i starts with empty environment
		{
			Name:      "ignore_env_short",
			Args:      []string{"-i", "FOO=bar", envBin},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R2.1: --ignore-environment long form
		{
			Name:      "ignore_env_long",
			Args:      []string{"--ignore-environment", "FOO=bar", envBin},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R2.1: -i with no command prints empty (or just assignments)
		{
			Name:      "ignore_env_no_command",
			Args:      []string{"-i"},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R2.1: -i with multiple assignments and command
		{
			Name:      "ignore_env_multi_vars",
			Args:      []string{"-i", "X=1", "Y=2", envBin},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R1.3: command not found exits 127
		{
			Name:      "command_not_found",
			Args:      []string{"nonexistent_command_xyz_12345"},
			ExitCode:  127,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// R1.2: exit code passthrough
		{
			Name:     "exit_code_passthrough",
			Args:     []string{"sh", "-c", "exit 42"},
			ExitCode: 42,
		},
		// R1.2: exit code zero passthrough
		{
			Name:     "exit_code_zero",
			Args:     []string{"true"},
			ExitCode: 0,
		},
		// R1.2: exit code one passthrough
		{
			Name:     "exit_code_one",
			Args:     []string{"false"},
			ExitCode: 1,
		},
		// -- terminates option processing
		{
			Name: "double_dash_stops_flags",
			Args: []string{"--", "echo", "hello"},
		},
		// R2.3: value containing equals sign
		{
			Name:      "value_with_equals",
			Args:      []string{"-i", "A=1=2=3", envBin},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R2.1 + R2.3: -i with no vars and no command prints nothing
		{
			Name: "ignore_env_empty",
			Args: []string{"-i"},
		},
		// R2.2: -u removes a variable
		{
			Name:      "unset_single",
			Args:      []string{"-u", "HOME", envBin},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R2.2: --unset=NAME long form
		{
			Name:      "unset_long_form",
			Args:      []string{"--unset=HOME", envBin},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R2.2: multiple -u flags
		{
			Name:      "unset_multiple",
			Args:      []string{"-u", "HOME", "-u", "USER", envBin},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R2.2: -u with -i (unset from empty env is a no-op)
		{
			Name:      "unset_with_ignore_env",
			Args:      []string{"-i", "-u", "HOME", "FOO=bar", envBin},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R2.2: -u variable that doesn't exist
		{
			Name:      "unset_nonexistent",
			Args:      []string{"-u", "NONEXISTENT_VAR_XYZ_12345", envBin},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R2.2: combined -iu flags
		{
			Name:      "combined_iu_flags",
			Args:      []string{"-iu", "HOME", "FOO=bar", envBin},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R3.1: -0 NUL-delimited output
		{
			Name:      "null_terminated_short",
			Args:      []string{"-i", "-0", "A=1", "B=2"},
			Normalize: []testutils.NormalizeFunc{sortNulLines},
		},
		// R3.1: --null NUL-delimited output
		{
			Name:      "null_terminated_long",
			Args:      []string{"-i", "--null", "A=1", "B=2"},
			Normalize: []testutils.NormalizeFunc{sortNulLines},
		},
		// R3.1: combined -i0 flags
		{
			Name:      "null_combined_i0",
			Args:      []string{"-i0", "X=hello", "Y=world"},
			Normalize: []testutils.NormalizeFunc{sortNulLines},
		},
		// R3.1: -0 with inherited environment
		{
			Name:      "null_with_inherited_env",
			Args:      []string{"-0"},
			Normalize: []testutils.NormalizeFunc{sortNulLines},
		},
		// R3.2: exit code passthrough with modified env
		{
			Name:     "exit_code_with_env_mod",
			Args:     []string{"FOO=bar", "sh", "-c", "exit 7"},
			ExitCode: 7,
		},
		// R3.3 + R4.3: invalid short option
		{
			Name:      "invalid_short_option",
			Args:      []string{"-z"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// R3.3 + R4.3: invalid long option
		{
			Name:      "invalid_long_option",
			Args:      []string{"--bogus"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// R3.3 + R4.3: -u missing argument
		{
			Name:      "unset_missing_argument",
			Args:      []string{"-u"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func findEnvBin(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("env")
	if err != nil {
		t.Fatal("env binary not found")
	}
	return path
}

var binaryNameRe = regexp.MustCompile(`(?:/[^ ']+/)?g?env([:' ])`)

func normalizeBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("env$1"))
}

func sortLines(b []byte) []byte {
	s := string(b)
	if s == "" {
		return b
	}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	sort.Strings(lines)
	return []byte(strings.Join(lines, "\n") + "\n")
}

func sortNulLines(b []byte) []byte {
	s := string(b)
	if s == "" {
		return b
	}
	s = strings.TrimSuffix(s, "\x00")
	lines := strings.Split(s, "\x00")
	sort.Strings(lines)
	return []byte(strings.Join(lines, "\x00") + "\x00")
}
