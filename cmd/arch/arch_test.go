// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd045-arch R1.1, R1.2, R2.1, R2.2, R3.1–R3.3:
// compare Go arch against garch reference binary.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// binaryNameNormalizer replaces the reference binary name and path in
// stderr so that "garch" and "/opt/.../garch" both become "arch".
var binaryNameNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	re := regexp.MustCompile(`[^\s']*g?arch`)
	return re.ReplaceAll(data, []byte("arch"))
}

// versionNormalizer replaces all version output with a fixed string so
// that GNU's multi-line copyright block and Go's single-line version match.
var versionNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	if len(data) > 0 {
		return []byte("VERSION OUTPUT")
	}
	return data
}

// helpNormalizer replaces all help output with a fixed string so that
// structural differences between GNU and Go help text don't cause failures.
var helpNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	if len(data) > 0 {
		return []byte("HELP OUTPUT")
	}
	return data
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("garch")
	if err != nil {
		t.Skipf("reference binary garch not in PATH: %v", err)
	}
	errNorm := []testutils.NormalizeFunc{binaryNameNormalizer}
	tests := []testutils.DiffTest{
		// R1.1, R1.2: normal invocation prints machine hardware name.
		{
			Name: "no_args",
			Args: []string{},
		},
		// R2.1: extra operand produces error and exit 1.
		{
			Name:      "extra_operand",
			Args:      []string{"extraarg"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R2.2: unknown flag produces error and exit 1.
		{
			Name:      "unknown_flag",
			Args:      []string{"--unknown"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R2.2: unknown short flag.
		{
			Name:      "unknown_short_flag",
			Args:      []string{"-x"},
			ExitCode:  1,
			Normalize: errNorm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestVersion verifies that --version prints version info to stdout
// and exits 0.
func TestVersion(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("garch")
	if err != nil {
		t.Skipf("reference binary garch not in PATH: %v", err)
	}
	verNorm := []testutils.NormalizeFunc{versionNormalizer}
	tests := []testutils.DiffTest{
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: verNorm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestHelp verifies that --help prints usage to stdout and exits 0.
func TestHelp(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("garch")
	if err != nil {
		t.Skipf("reference binary garch not in PATH: %v", err)
	}
	helpNorm := []testutils.NormalizeFunc{helpNormalizer}
	tests := []testutils.DiffTest{
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: helpNorm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
