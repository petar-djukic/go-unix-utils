// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd044-uname R4.1, R4.2, R4.3 (differential tests for R1.1-R1.9, R2.1, R2.2, R3.1)
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for uname.
const refBinaryName = "guname"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// normalizeProgramName replaces the reference binary path/name with the
	// Go binary name so stderr messages compare equal despite different argv[0].
	programNamePattern := regexp.MustCompile(`(?:` + regexp.QuoteMeta(refBin) + `|guname|uname)`)
	normalizeProgramName := func(b []byte) []byte {
		return programNamePattern.ReplaceAll(b, []byte("PROG"))
	}

	// normalizeTryPath strips absolute path from "Try '...' --help" messages.
	tryPattern := regexp.MustCompile(`Try '[^']*'`)
	normalizeTryPath := func(b []byte) []byte {
		return tryPattern.ReplaceAll(b, []byte("Try 'PROG'"))
	}

	stderrNorm := []testutils.NormalizeFunc{normalizeProgramName, normalizeTryPath}

	tests := []testutils.DiffTest{
		// R1.1: no arguments — prints kernel name.
		{
			Name: "no_args",
			Args: []string{},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: -s flag — kernel name.
		{
			Name: "flag_s",
			Args: []string{"-s"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: -n flag — node hostname.
		{
			Name: "flag_n",
			Args: []string{"-n"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: -r flag — kernel release.
		{
			Name: "flag_r",
			Args: []string{"-r"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.5: -v flag — kernel version string.
		{
			Name: "flag_v",
			Args: []string{"-v"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.6: -m flag — machine hardware name.
		{
			Name: "flag_m",
			Args: []string{"-m"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.7: -p flag — processor type.
		{
			Name: "flag_p",
			Args: []string{"-p"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.8: -i flag — hardware platform.
		{
			Name: "flag_i",
			Args: []string{"-i"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.9: -o flag — operating system name.
		{
			Name: "flag_o",
			Args: []string{"-o"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -a flag — all fields in canonical order.
		{
			Name: "flag_a",
			Args: []string{"-a"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: combined flags — fields in canonical order.
		{
			Name: "combined_snr",
			Args: []string{"-snr"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: combined flags including new flags.
		{
			Name: "combined_vm",
			Args: []string{"-vm"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: combined flags — all individual flags.
		{
			Name: "combined_snrvmpi",
			Args: []string{"-snrvmpi"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: combined flags including -o.
		{
			Name: "combined_so",
			Args: []string{"-so"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: combined flags — all individual flags including -o.
		{
			Name: "combined_snrvmpio",
			Args: []string{"-snrvmpio"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: extra operand — error on stderr, exit 1.
		{
			Name:      "extra_operand",
			Args:      []string{"extraarg"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// R3.1: extra operand after valid flag — error on stderr, exit 1.
		{
			Name:      "extra_operand_after_flag",
			Args:      []string{"-s", "extraarg"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// R3.2: unknown flag — error on stderr, exit 1.
		{
			Name:      "unknown_flag",
			Args:      []string{"-z"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffHelpVersion verifies --help and --version exit 0.
// Output content differs between implementations, so stdout/stderr are
// normalized to empty; only exit codes are compared.
func TestDiffHelpVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	clearOutput := func(b []byte) []byte { return nil }

	tests := []testutils.DiffTest{
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
