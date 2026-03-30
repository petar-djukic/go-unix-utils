// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// who_test.go implements differential tests for cmd/who against gwho.
// Covers prd097-who R1.1-R1.4, R2.1-R2.4.

package main_test

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// binaryNameRe matches gwho or who with optional path prefix.
var binaryNameRe = regexp.MustCompile(`(/[^ ']*/)?(g?who)`)

// normalizeBinaryName replaces binary path references with "who"
// so stderr from gwho and our binary can be compared.
func normalizeBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("who"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwho")
	if err != nil {
		t.Skip("reference binary gwho not in PATH")
	}

	errorNorm := []testutils.NormalizeFunc{normalizeBinaryName}

	tests := []testutils.DiffTest{
		{
			// R1.1: prints logged-in users with terminal and time.
			Name: "who_default",
			Args: []string{},
		},
		{
			// R1.3: "who am i" prints only the current terminal entry.
			Name: "who_am_i",
			Args: []string{"am", "i"},
		},
		{
			// R2.1: -H prints a header line above the output.
			Name: "who_heading",
			Args: []string{"-H"},
		},
		{
			// R2.3: -b prints the time of the last system boot.
			Name: "who_boot",
			Args: []string{"-b"},
		},
		{
			// R2.4: -q prints login names and count.
			Name: "who_count",
			Args: []string{"-q"},
		},
		{
			// R2.1+R2.3: combined flags -Hb.
			Name: "who_heading_boot",
			Args: []string{"-Hb"},
		},
		{
			// R2.2: unknown long flag produces error.
			Name:      "who_unknown_flag",
			Args:      []string{"--bogus"},
			ExitCode:  1,
			Normalize: errorNorm,
		},
		{
			// R2.2: invalid short flag produces error.
			Name:      "who_invalid_short",
			Args:      []string{"-x"},
			ExitCode:  1,
			Normalize: errorNorm,
		},
		{
			// R1.4: extra operand produces error.
			Name:      "who_extra_operand",
			Args:      []string{"a", "b", "c"},
			ExitCode:  1,
			Normalize: errorNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
