// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests: prd039-env R2.2, R2.3, R3.1, R3.2, R3.3, R4.1, R4.2, R4.3
package main

import (
	"bytes"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeProgramName normalizes stderr by replacing absolute paths to genv
// or env binaries with just "env", and replacing "genv:" with "env:", so that
// program-name and path differences do not cause false failures.
func normalizeProgramName(b []byte) []byte {
	s := string(b)
	// Replace any absolute path ending in /genv (e.g. '/opt/homebrew/bin/genv')
	// with just "env". Walk backwards from "/genv" to find the path start.
	for {
		idx := strings.Index(s, "/genv")
		if idx < 0 {
			break
		}
		start := idx
		for start > 0 && s[start-1] != '\'' && s[start-1] != ' ' && s[start-1] != '"' && s[start-1] != '\n' {
			start--
		}
		if start < idx && s[start] == '/' {
			s = s[:start] + "env" + s[idx+len("/genv"):]
		} else {
			s = s[:idx] + "/env" + s[idx+len("/genv"):]
		}
	}
	// Also replace any remaining bare "genv:" with "env:" for non-path cases.
	s = strings.ReplaceAll(s, "genv:", "env:")
	// Replace absolute paths to the Go binary (e.g. '/path/to/env') with "env".
	for {
		idx := strings.Index(s, "/env:")
		if idx < 0 {
			break
		}
		start := idx
		for start > 0 && s[start-1] != '\'' && s[start-1] != ' ' && s[start-1] != '"' && s[start-1] != '\n' {
			start--
		}
		if start < idx && s[start] == '/' {
			s = s[:start] + "env" + s[idx+len("/env"):]
		} else {
			break
		}
	}
	return []byte(s)
}

// sortLines is a NormalizeFunc that sorts output lines so environment variable
// ordering differences between Go and genv do not cause false failures.
func sortLines(b []byte) []byte {
	s := string(b)
	if s == "" {
		return b
	}
	// Detect terminator: NUL or newline.
	var terminator string
	if bytes.Contains(b, []byte{0}) {
		terminator = "\x00"
	} else {
		terminator = "\n"
	}
	// Remove trailing terminator to avoid empty last element.
	s = strings.TrimSuffix(s, terminator)
	lines := strings.Split(s, terminator)
	sort.Strings(lines)
	return []byte(strings.Join(lines, terminator) + terminator)
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("genv")
	if err != nil {
		t.Skipf("reference binary genv not in PATH: %v", err)
	}

	envSort := []testutils.NormalizeFunc{sortLines}

	tests := []testutils.DiffTest{
		// R4.2: No-argument environment dump.
		{
			Name:      "no arguments prints environment",
			Args:      []string{},
			Env:       []string{"LC_ALL=C"},
			Normalize: envSort,
		},

		// R4.2: -i empty environment dump.
		{
			Name:      "ignore env prints empty",
			Args:      []string{"-i"},
			Env:       []string{"LC_ALL=C"},
			Normalize: envSort,
		},
		{
			Name:      "ignore env with assignment only",
			Args:      []string{"-i", "FOO=bar"},
			Env:       []string{"LC_ALL=C"},
			Normalize: envSort,
		},

		// R2.2: -u unsets a variable.
		{
			Name:      "unset single variable",
			Args:      []string{"-u", "HOME"},
			Env:       []string{"LC_ALL=C"},
			Normalize: envSort,
		},
		{
			Name:      "unset multiple variables",
			Args:      []string{"-u", "HOME", "-u", "USER"},
			Env:       []string{"LC_ALL=C"},
			Normalize: envSort,
		},
		{
			Name:      "unset nonexistent variable",
			Args:      []string{"-u", "NONEXISTENT_VAR_XYZ"},
			Env:       []string{"LC_ALL=C"},
			Normalize: envSort,
		},
		{
			Name:      "unset with long flag",
			Args:      []string{"--unset=HOME"},
			Env:       []string{"LC_ALL=C"},
			Normalize: envSort,
		},

		// R2.3: NAME=VALUE sets or overrides variables.
		{
			Name:      "set single variable and print",
			Args:      []string{"FOO=bar"},
			Env:       []string{"LC_ALL=C"},
			Normalize: envSort,
		},
		{
			Name:      "override existing variable",
			Args:      []string{"HOME=/tmp/override"},
			Env:       []string{"LC_ALL=C"},
			Normalize: envSort,
		},
		{
			Name:      "set multiple variables",
			Args:      []string{"FOO=bar", "BAZ=qux"},
			Env:       []string{"LC_ALL=C"},
			Normalize: envSort,
		},
		{
			Name:      "set variable with empty value",
			Args:      []string{"FOO="},
			Env:       []string{"LC_ALL=C"},
			Normalize: envSort,
		},
		{
			Name:      "set variable with equals in value",
			Args:      []string{"FOO=bar=baz"},
			Env:       []string{"LC_ALL=C"},
			Normalize: envSort,
		},

		// R2.3 + R2.2: combined unset and set.
		{
			Name:      "unset then set same variable",
			Args:      []string{"-u", "HOME", "HOME=/new/home"},
			Env:       []string{"LC_ALL=C"},
			Normalize: envSort,
		},

		// R3.1: -0 / --null NUL-delimited output.
		{
			Name:      "null terminated output with -0",
			Args:      []string{"-i", "-0", "FOO=bar", "BAZ=qux"},
			Env:       []string{"LC_ALL=C"},
			Normalize: envSort,
		},
		{
			Name:      "null terminated output with --null",
			Args:      []string{"-i", "--null", "A=1"},
			Env:       []string{"LC_ALL=C"},
			Normalize: envSort,
		},

		// R3.2: exit code passthrough from COMMAND.
		{
			Name:     "command exit code 0",
			Args:     []string{"true"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		{
			Name:     "command exit code 1",
			Args:     []string{"false"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		{
			Name:     "command exit code passthrough",
			Args:     []string{"sh", "-c", "exit 42"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 42,
		},
		{
			Name:      "command not found",
			Args:      []string{"nonexistent_command_xyz_12345"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  127,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},

		// R2.2 + R2.3 + COMMAND: modified env passed to command.
		{
			Name: "set var and run command",
			Args: []string{"FOO=hello", "sh", "-c", "echo $FOO"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "unset var and run command",
			Args: []string{"-u", "HOME", "sh", "-c", "echo HOME=${HOME-unset}"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name:     "empty env run command",
			Args:     []string{"-i", "FOO=bar", "sh", "-c", "echo $FOO"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},

		// R3.3 / R4.3: Invalid short option.
		{
			Name:      "invalid short option",
			Args:      []string{"-x"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},

		// R3.3 / R4.3: Invalid long option.
		{
			Name:      "invalid long option",
			Args:      []string{"--invalid"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},

		// R4.3: Missing -u argument.
		{
			Name:      "missing unset argument",
			Args:      []string{"-u"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},

		// R4.3: Command not found (exit 127) — also exercises R1.3.
		{
			Name:      "command not found exit 127",
			Args:      []string{"nonexistent_cmd_abc_99999"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  127,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
