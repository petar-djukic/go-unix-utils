// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/env against the GNU reference binary (genv).
//
// Implements prd039-env acceptance criteria AC1-AC8 via testutils.RunDiffTests.
package main

import (
	"bytes"
	"os/exec"
	"sort"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrBinaryNameNormalizer replaces "genv:" with "env:" so stderr
// comparison ignores the binary name difference.
var stderrBinaryNameNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("genv:"), []byte("env:"))
}

// sortLinesNormalizer sorts output lines so insertion-order differences
// between Go and GNU env do not cause false failures.
var sortLinesNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	b = bytes.TrimRight(b, "\n")
	if len(b) == 0 {
		return b
	}
	lines := bytes.Split(b, []byte("\n"))
	sort.Slice(lines, func(i, j int) bool {
		return bytes.Compare(lines[i], lines[j]) < 0
	})
	result := bytes.Join(lines, []byte("\n"))
	return append(result, '\n')
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("genv")
	if err != nil {
		t.Skipf("reference binary genv not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: No arguments prints all environment variables.
		{
			Name:      "env_no_args",
			Args:      []string{},
			Normalize: []testutils.NormalizeFunc{sortLinesNormalizer},
		},
		// R2.1: -i starts with empty environment.
		{
			Name: "env_ignore_env",
			Args: []string{"-i"},
		},
		// R2.3: NAME=VALUE sets a variable.
		{
			Name: "env_set_var",
			Args: []string{"-i", "FOO=bar"},
		},
		// R2.3: Multiple NAME=VALUE pairs.
		{
			Name:      "env_set_multiple_vars",
			Args:      []string{"-i", "FOO=bar", "BAZ=qux"},
			Normalize: []testutils.NormalizeFunc{sortLinesNormalizer},
		},
		// R2.2: -u removes a variable before printing.
		{
			Name:      "env_unset_var",
			Args:      []string{"-i", "-u", "FOO", "FOO=bar", "BAZ=qux"},
			Normalize: []testutils.NormalizeFunc{sortLinesNormalizer},
		},
		// R3.1: -0 terminates with NUL.
		{
			Name: "env_null_terminator",
			Args: []string{"-i", "-0", "FOO=bar"},
		},
		// R1.2: Execute a command.
		{
			Name: "env_exec_command",
			Args: []string{"-i", "FOO=hello", "/bin/sh", "-c", "echo $FOO"},
		},
		// R1.3: Command not found, exit 127.
		{
			Name:      "env_command_not_found",
			Args:      []string{"surely_nonexistent_command_xyz"},
			ExitCode:  127,
			Normalize: []testutils.NormalizeFunc{stderrBinaryNameNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
