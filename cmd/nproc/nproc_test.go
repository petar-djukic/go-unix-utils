// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/nproc against gnproc (GNU coreutils).
// Implements prd046-nproc R3.1-R3.3 test coverage.
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
	re := regexp.MustCompile(`[^\s']*[/\\]?[gG]?nproc`)
	return re.ReplaceAll(b, []byte("PROG"))
}

// versionHelpNormalizer blanks stdout content for --version and --help tests.
// GNU and Go implementations produce structurally different output (different
// suite names, version numbers, copyright text, help wording), so byte-level
// comparison is not meaningful. The differential value is verifying both exit 0
// and write to stdout (not stderr).
var versionHelpNormalizer = func(b []byte) []byte {
	if len(b) > 0 {
		return []byte("OUTPUT\n")
	}
	return b
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnproc")
	if err != nil {
		t.Skipf("reference binary gnproc not in PATH: %v", err)
	}

	// R3.3: all differential tests set LC_ALL=C.
	lcEnv := []string{"LC_ALL=C"}

	tests := []testutils.DiffTest{
		// R1.1, R3.1, R3.2: no arguments — prints available CPU count.
		{
			Name:     "R1.1_no_args_cpu_count",
			Env:      lcEnv,
			ExitCode: 0,
		},
		// R1.2, R3.2: --all prints installed processor count.
		{
			Name:     "R1.2_all_flag",
			Args:     []string{"--all"},
			Env:      lcEnv,
			ExitCode: 0,
		},
		// R1.3, R3.2: --ignore=1 subtracts 1 from count.
		{
			Name:     "R1.3_ignore_one",
			Args:     []string{"--ignore=1"},
			Env:      lcEnv,
			ExitCode: 0,
		},
		// R1.4, R3.2: --all --ignore=1 subtracts from installed count.
		{
			Name:     "R1.4_all_ignore_combined",
			Args:     []string{"--all", "--ignore=1"},
			Env:      lcEnv,
			ExitCode: 0,
		},
		// R1.3: --ignore with large value floors at 1.
		{
			Name:     "R1.3_ignore_large_floors_at_one",
			Args:     []string{"--ignore=9999"},
			Env:      lcEnv,
			ExitCode: 0,
		},
		// R1.4: OMP_NUM_THREADS overrides default count.
		{
			Name:     "R1.4_omp_num_threads",
			Env:      append(lcEnv, "OMP_NUM_THREADS=2"),
			ExitCode: 0,
		},
		// R1.4: --all ignores OMP_NUM_THREADS.
		{
			Name:     "R1.4_omp_ignored_with_all",
			Args:     []string{"--all"},
			Env:      append(lcEnv, "OMP_NUM_THREADS=2"),
			ExitCode: 0,
		},
		// R1.4: OMP_NUM_THREADS with comma-separated values uses first.
		{
			Name:     "R1.4_omp_comma_separated",
			Env:      append(lcEnv, "OMP_NUM_THREADS=3,5"),
			ExitCode: 0,
		},
		// R2.2: --ignore interacts with OMP_NUM_THREADS — subtraction applies after override.
		{
			Name:     "R2.2_omp_with_ignore",
			Args:     []string{"--ignore=1"},
			Env:      append(lcEnv, "OMP_NUM_THREADS=4"),
			ExitCode: 0,
		},
		// R2.3: --all --ignore with OMP_NUM_THREADS — subtraction from runtime, OMP ignored.
		{
			Name:     "R2.3_all_ignore_omp_combined",
			Args:     []string{"--all", "--ignore=1"},
			Env:      append(lcEnv, "OMP_NUM_THREADS=4"),
			ExitCode: 0,
		},
		// R2.2: --ignore floors at 1 even with OMP_NUM_THREADS.
		{
			Name:     "R2.2_omp_ignore_large_floors",
			Args:     []string{"--ignore=9999"},
			Env:      append(lcEnv, "OMP_NUM_THREADS=4"),
			ExitCode: 0,
		},
		// R2.1: extra operand produces error.
		{
			Name:      "R2.1_extra_operand",
			Args:      []string{"foo"},
			Env:       lcEnv,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{binaryPathNormalizer},
		},
		// R2.2: --ignore with non-numeric value produces error.
		{
			Name:      "R2.2_ignore_non_numeric",
			Args:      []string{"--ignore=abc"},
			Env:       lcEnv,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{binaryPathNormalizer},
		},
		// R2.3: unknown long option produces error.
		{
			Name:      "R2.3_unknown_long_option",
			Args:      []string{"--unknown"},
			Env:       lcEnv,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{binaryPathNormalizer},
		},
		// R2.3: unknown short flag produces error.
		{
			Name:      "R2.3_unknown_short_flag",
			Args:      []string{"-z"},
			Env:       lcEnv,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{binaryPathNormalizer},
		},
		// R3.3: --version exits 0 and produces stdout output.
		{
			Name:      "R3.3_version_flag",
			Args:      []string{"--version"},
			Env:       lcEnv,
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{versionHelpNormalizer},
		},
		// R3.3: --help exits 0 and produces stdout output.
		{
			Name:      "R3.3_help_flag",
			Args:      []string{"--help"},
			Env:       lcEnv,
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{versionHelpNormalizer},
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
