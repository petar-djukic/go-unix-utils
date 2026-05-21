// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfactor")
	if err != nil {
		t.Skip("reference binary not found")
	}

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?factor`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("factor"))
	})

	discardStdout := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	tests := []testutils.DiffTest{
		{
			Name:      "help",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
		{
			Name:      "version",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
		{
			Name: "single_composite",
			Args: []string{"12"},
		},
		{
			Name: "number_one",
			Args: []string{"1"},
		},
		{
			Name: "prime_number",
			Args: []string{"97"},
		},
		{
			Name: "multiple_arguments",
			Args: []string{"6", "7", "12", "1", "97"},
		},
		{
			Name: "large_composite",
			Args: []string{"1000000"},
		},
		{
			Name: "power_of_two",
			Args: []string{"1024"},
		},
		{
			Name: "prime_squared",
			Args: []string{"49"},
		},
		{
			Name: "two",
			Args: []string{"2"},
		},
		{
			Name: "zero",
			Args: []string{"0"},
		},
		{
			Name:  "stdin_single",
			Stdin: []byte("15\n"),
		},
		{
			Name:  "stdin_multiple",
			Stdin: []byte("6\n7\n12\n1\n97\n"),
		},
		{
			Name:  "stdin_blank_lines",
			Stdin: []byte("15\n\n\n97\n"),
		},
		{
			Name:  "stdin_large_number",
			Stdin: []byte("9223372036854775783\n"),
		},
		{
			Name:      "error_non_integer",
			Args:      []string{"abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name:      "error_negative_stdin",
			Stdin:     []byte("-5\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name:      "stdin_mixed_valid_invalid",
			Stdin:     []byte("12\nabc\n97\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
