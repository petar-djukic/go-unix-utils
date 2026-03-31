// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// who_test.go implements differential tests for cmd/who against gwho.
// Covers prd097-who R1.1-R1.4, R2.1-R2.4, R3.1-R3.3.

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
		// R1.1: default listing of logged-in users.
		{
			Name: "who_default",
			Args: []string{},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: "who am i" prints only the current terminal entry.
		{
			Name: "who_am_i",
			Args: []string{"am", "i"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -H prints a header line above the output.
		{
			Name: "who_heading",
			Args: []string{"-H"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: -u shows idle time for each user.
		{
			Name: "who_users_idle",
			Args: []string{"-u"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: -b prints the time of the last system boot.
		{
			Name: "who_boot",
			Args: []string{"-b"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.4: -q prints login names and count.
		{
			Name: "who_count",
			Args: []string{"-q"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: combined -H and -b flags.
		{
			Name: "who_heading_boot",
			Args: []string{"-Hb"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: combined -H and -u flags (heading with idle columns).
		{
			Name: "who_heading_users",
			Args: []string{"-Hu"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: combined -b and -u flags (boot + users with idle).
		{
			Name: "who_boot_users",
			Args: []string{"-bu"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: all type flags combined -Hbu (heading + boot + idle).
		{
			Name: "who_heading_boot_users",
			Args: []string{"-Hbu"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: -q combined with -H (count mode ignores heading).
		{
			Name: "who_count_heading",
			Args: []string{"-qH"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: exit 0 on success (verified by default ExitCode=0).
		// Also covers long --heading flag.
		{
			Name: "who_long_heading",
			Args: []string{"--heading"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: long --boot flag exits 0 on success.
		{
			Name: "who_long_boot",
			Args: []string{"--boot"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: long --count flag exits 0 on success.
		{
			Name: "who_long_count",
			Args: []string{"--count"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: long --users flag exits 0 on success.
		{
			Name: "who_long_users",
			Args: []string{"--users"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: exit 1 for unknown long flag.
		{
			Name:      "who_unknown_flag",
			Args:      []string{"--bogus"},
			ExitCode:  1,
			Normalize: errorNorm,
		},
		// R3.2: exit 1 for invalid short flag.
		{
			Name:      "who_invalid_short",
			Args:      []string{"-x"},
			ExitCode:  1,
			Normalize: errorNorm,
		},
		// R1.4: exit 1 for extra operand.
		{
			Name:      "who_extra_operand",
			Args:      []string{"a", "b", "c"},
			ExitCode:  1,
			Normalize: errorNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
