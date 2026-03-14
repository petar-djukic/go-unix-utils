// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests: prd040-printenv R2.2, R2.3, R2.4, R3.1
package main

import (
	"bytes"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// sortLines is a NormalizeFunc that sorts output lines so environment variable
// ordering differences between Go and gprintenv do not cause false failures.
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
	refBin, err := exec.LookPath("gprintenv")
	if err != nil {
		t.Skipf("reference binary gprintenv not in PATH: %v", err)
	}

	envSort := []testutils.NormalizeFunc{sortLines}

	tests := []testutils.DiffTest{
		// R2.4: No-argument full dump always exits 0.
		{
			Name:      "no arguments prints all env vars",
			Args:      []string{},
			Env:       []string{"LC_ALL=C"},
			Normalize: envSort,
		},

		// R2.2: Single existing variable prints value and exits 0.
		{
			Name: "single existing variable",
			Args: []string{"HOME"},
			Env:  []string{"LC_ALL=C"},
		},

		// R2.2: Multiple existing variables print each value on a separate line.
		{
			Name: "multiple existing variables",
			Args: []string{"HOME", "PATH"},
			Env:  []string{"LC_ALL=C"},
		},

		// R2.3: Missing variable exits 1.
		{
			Name:     "missing variable exits 1",
			Args:     []string{"NONEXISTENT_VAR_XYZ_98765"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},

		// R2.3: Mix of existing and missing variables exits 1.
		{
			Name:     "mix existing and missing exits 1",
			Args:     []string{"HOME", "NONEXISTENT_VAR_XYZ_98765"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},

		// R2.3: Mix of missing and existing (reversed order) exits 1.
		{
			Name:     "missing then existing exits 1",
			Args:     []string{"NONEXISTENT_VAR_XYZ_98765", "HOME"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},

		// R2.3: All missing variables exits 1.
		{
			Name:     "all missing variables exits 1",
			Args:     []string{"NONEXISTENT_VAR_A_12345", "NONEXISTENT_VAR_B_12345"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},

		// R3.1: -0 NUL-delimited output for single variable.
		{
			Name: "null terminated single variable",
			Args: []string{"-0", "HOME"},
			Env:  []string{"LC_ALL=C"},
		},

		// R3.1: --null long flag NUL-delimited output.
		{
			Name: "null terminated with long flag",
			Args: []string{"--null", "HOME"},
			Env:  []string{"LC_ALL=C"},
		},

		// R3.1: -0 NUL-delimited output for multiple variables.
		{
			Name: "null terminated multiple variables",
			Args: []string{"-0", "HOME", "PATH"},
			Env:  []string{"LC_ALL=C"},
		},

		// R3.1: -0 with no arguments (full dump).
		{
			Name:      "null terminated full dump",
			Args:      []string{"-0"},
			Env:       []string{"LC_ALL=C"},
			Normalize: envSort,
		},

		// R3.1 + R2.3: -0 with missing variable still exits 1.
		{
			Name:     "null terminated missing variable exits 1",
			Args:     []string{"-0", "NONEXISTENT_VAR_XYZ_98765"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},

		// R3.3: No error message on stderr for missing variables.
		{
			Name:     "no stderr for missing variable",
			Args:     []string{"NONEXISTENT_VAR_XYZ_98765"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
