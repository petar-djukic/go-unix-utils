// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/seq against gseq (Homebrew GNU coreutils).
// Implements prd019-seq R4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinaryName = "gseq"

// blankOutput blanks output so only exit codes are compared.
// Used for --help, --version, and error messages where text differs.
var blankOutput testutils.NormalizeFunc = func(b []byte) []byte { return nil }

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R1.1: Single argument form (seq LAST).
		{
			Name: "single arg",
			Args: []string{"5"},
		},
		// R1.1: Two argument form (seq FIRST LAST).
		{
			Name: "two args",
			Args: []string{"2", "5"},
		},
		// R1.1, R1.2: Three argument form (seq FIRST STEP LAST).
		{
			Name: "three args",
			Args: []string{"1", "2", "10"},
		},
		// R1.2: Descending sequence.
		{
			Name: "descending",
			Args: []string{"5", "-1", "1"},
		},
		// R1.3: FIRST equals LAST.
		{
			Name: "first equals last",
			Args: []string{"3", "3"},
		},
		// R1.4, R4.1: Empty sequence (FIRST > LAST with positive step).
		{
			Name: "empty ascending",
			Args: []string{"5", "1"},
		},
		// R1.4: Empty sequence (negative step, FIRST < LAST).
		{
			Name: "empty descending",
			Args: []string{"1", "-1", "5"},
		},
		// R2.3: Floating-point sequence.
		{
			Name: "floating point",
			Args: []string{"0.1", "0.1", "0.5"},
		},
		// R2.2: Custom separator.
		{
			Name: "custom separator",
			Args: []string{"-s", ", ", "1", "3"},
		},
		// R2.2: Separator with --separator= form.
		{
			Name: "separator long form",
			Args: []string{"--separator=:", "1", "4"},
		},
		// R3.3: Equal width.
		{
			Name: "equal width",
			Args: []string{"-w", "8", "12"},
		},
		// R3.1: Format string.
		{
			Name: "format string",
			Args: []string{"-f", "%.2f", "1", "3"},
		},
		// R3.1: Format string with --format= form.
		{
			Name: "format long form",
			Args: []string{"--format=%.3f", "1", "3"},
		},
		// R1.1: Single argument with large number.
		{
			Name: "single arg large",
			Args: []string{"3"},
		},
		// Negative first and last.
		{
			Name: "negative range",
			Args: []string{"-3", "1", "3"},
		},
		// R2.2: Separator with tab.
		{
			Name: "tab separator",
			Args: []string{"-s", "\t", "1", "3"},
		},
		// Step of 0.5.
		{
			Name: "half step",
			Args: []string{"1", "0.5", "3"},
		},
		// R1.5: Zero step error.
		{
			Name:      "zero step error",
			Args:      []string{"1", "0", "5"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{blankOutput},
		},
		// R3.2: Invalid format specifier.
		{
			Name:      "invalid format error",
			Args:      []string{"-f", "%s", "1", "3"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{blankOutput},
		},
		// R4.2: Non-numeric argument.
		{
			Name:      "non numeric error",
			Args:      []string{"abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{blankOutput},
		},
		// No arguments error.
		{
			Name:      "no arguments error",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{blankOutput},
		},
		// --help and --version: blank output, only exit code matters.
		{
			Name:      "help flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{blankOutput},
		},
		{
			Name:      "version flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{blankOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
