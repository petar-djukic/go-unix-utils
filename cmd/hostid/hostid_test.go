// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/hostid against ghostid (GNU coreutils).
// Implements prd048-hostid R3.1-R3.3 test coverage.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// binaryPathNormalizer replaces binary paths in error messages with "PROG"
// so that stderr comparison is path-independent.
var binaryPathNormalizer = func(b []byte) []byte {
	re := regexp.MustCompile(`[^\s']*[/\\]?[gG]?hostid`)
	return re.ReplaceAll(b, []byte("PROG"))
}

// versionNormalizer replaces the entire --version output with a fixed string
// so that version differences between Go and GNU binaries do not cause false
// divergence. GNU --version prints version, license, and author lines; the
// Go binary prints only the version line. Both exit 0.
var versionNormalizer = func(b []byte) []byte {
	return []byte("VERSION\n")
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ghostid")
	if err != nil {
		t.Skipf("reference binary ghostid not in PATH: %v", err)
	}

	// R3.3: all differential tests set LC_ALL=C.
	lcEnv := []string{"LC_ALL=C"}

	tests := []testutils.DiffTest{
		// R1.1, R3.1, R3.2: no arguments — prints host identifier.
		{
			Name:     "R1.1_no_args_hostid",
			Env:      lcEnv,
			ExitCode: 0,
		},
		// R2.1, R3.1, R3.2: extra positional operand produces error and exits 1.
		{
			Name:      "R2.1_extra_operand",
			Args:      []string{"foo"},
			Env:       lcEnv,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{binaryPathNormalizer},
		},
		// R2.2, R3.1, R3.2: unknown long option produces error and exits 1.
		{
			Name:      "R2.2_invalid_long_option",
			Args:      []string{"--invalid"},
			Env:       lcEnv,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{binaryPathNormalizer},
		},
		// R2.2, R3.1, R3.2: unknown short flag produces error and exits 1.
		{
			Name:      "R2.2_invalid_short_flag",
			Args:      []string{"-z"},
			Env:       lcEnv,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{binaryPathNormalizer},
		},
		// R1.2, R3.2: --version prints version info and exits 0.
		{
			Name:      "R1.2_version_flag",
			Args:      []string{"--version"},
			Env:       lcEnv,
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{versionNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestHelpExitsZero(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("unexpected error running %s --help: %v", goBin, err)
	}
	if len(out) == 0 {
		t.Error("--help produced no output")
	}
}

func TestVersionExitsZero(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("unexpected error running %s --version: %v", goBin, err)
	}
	if len(out) == 0 {
		t.Error("--version produced no output")
	}
}
