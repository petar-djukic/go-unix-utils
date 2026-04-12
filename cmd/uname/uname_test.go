// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/uname.
// Tests cover srd044-uname R3.2, R4.1, R4.2, R4.3.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgRe matches the program name/path prefix before a colon at line start.
var stderrProgRe = regexp.MustCompile(`(?m)^[^\s:]+:`)

// stderrTryRe matches the quoted program reference in Try hint lines.
var stderrTryRe = regexp.MustCompile(`'[^']*--help'`)

// stderrNormalizer normalizes program name differences in error messages.
// R3.2: replaces binary paths with "PROG" so error message structure
// can be compared between Go and GNU binaries.
func stderrNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	b = stderrProgRe.ReplaceAll(b, []byte("PROG:"))
	b = stderrTryRe.ReplaceAll(b, []byte("'PROG --help'"))
	return b
}

// discardOutput normalizes by discarding all output, used when
// output content differs by design (--version, --help) and only
// exit code comparison is meaningful.
func discardOutput(b []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guname")
	if err != nil {
		t.Skipf("reference binary guname not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.1/R4.2: default invocation with no arguments (equivalent to -s).
		{
			Name: "default_no_args",
			Args: []string{},
		},

		// R4.1: -s flag prints kernel name.
		{
			Name: "flag_s",
			Args: []string{"-s"},
		},

		// R4.1: -n flag prints network node hostname.
		{
			Name: "flag_n",
			Args: []string{"-n"},
		},

		// R4.1: -r flag prints kernel release.
		{
			Name: "flag_r",
			Args: []string{"-r"},
		},

		// R4.1: -v flag prints kernel version.
		{
			Name: "flag_v",
			Args: []string{"-v"},
		},

		// R4.1: -m flag prints machine hardware name.
		{
			Name: "flag_m",
			Args: []string{"-m"},
		},

		// R4.1: -p flag prints processor type.
		{
			Name: "flag_p",
			Args: []string{"-p"},
		},

		// R4.1: -i flag prints hardware platform.
		{
			Name: "flag_i",
			Args: []string{"-i"},
		},

		// R4.1: -o flag prints operating system.
		{
			Name: "flag_o",
			Args: []string{"-o"},
		},

		// R4.1: -a flag prints all fields.
		{
			Name: "flag_a",
			Args: []string{"-a"},
		},

		// R4.2: combined flags -sn prints kernel name and node name.
		{
			Name: "combined_sn",
			Args: []string{"-sn"},
		},

		// R4.2: combined flags -sr prints kernel name and release.
		{
			Name: "combined_sr",
			Args: []string{"-sr"},
		},

		// R4.2: combined flags -snrvm prints five fields.
		{
			Name: "combined_snrvm",
			Args: []string{"-snrvm"},
		},

		// R4.2: combined flags -mo prints machine and operating system.
		{
			Name: "combined_mo",
			Args: []string{"-mo"},
		},

		// R4.3: --help flag exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R4.3: --version flag exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R3.2/R4.3: unrecognized flag produces error and exit 1.
		{
			Name:      "unknown_flag",
			Args:      []string{"--bogus"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R3.2/R4.3: unknown short flag produces error and exit 1.
		{
			Name:      "unknown_short_flag",
			Args:      []string{"-z"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
