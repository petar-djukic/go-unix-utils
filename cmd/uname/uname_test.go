// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/uname against guname (GNU coreutils).
// Implements prd044-uname R1.1-R1.4, R4.1-R4.3 test coverage.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guname")
	if err != nil {
		t.Skipf("reference binary guname not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1, R4.3: no arguments — prints kernel name (equivalent to -s).
		{
			Name:     "R1.1_no_args_default_sysname",
			ExitCode: 0,
		},
		// R1.2: -s prints the kernel name.
		{
			Name:     "R1.2_flag_s_kernel_name",
			Args:     []string{"-s"},
			ExitCode: 0,
		},
		// R1.3: -n prints the network node hostname.
		{
			Name:     "R1.3_flag_n_nodename",
			Args:     []string{"-n"},
			ExitCode: 0,
		},
		// R1.4: -r prints the kernel release string.
		{
			Name:     "R1.4_flag_r_kernel_release",
			Args:     []string{"-r"},
			ExitCode: 0,
		},
		// R2.2: combined flags -sn prints sysname and nodename in canonical order.
		{
			Name:     "R2.2_combined_sn",
			Args:     []string{"-sn"},
			ExitCode: 0,
		},
		// R2.2: combined flags -sr prints sysname and release in canonical order.
		{
			Name:     "R2.2_combined_sr",
			Args:     []string{"-sr"},
			ExitCode: 0,
		},
		// R2.2: combined flags -nr prints nodename and release in canonical order.
		{
			Name:     "R2.2_combined_nr",
			Args:     []string{"-nr"},
			ExitCode: 0,
		},
		// R2.2: all three flags -snr prints all in canonical order.
		{
			Name:     "R2.2_combined_snr",
			Args:     []string{"-snr"},
			ExitCode: 0,
		},
		// R2.2: separate flags produce same result as combined.
		{
			Name:     "R2.2_separate_s_n_r",
			Args:     []string{"-s", "-n", "-r"},
			ExitCode: 0,
		},
		// R2.2: reverse order flags still print in canonical order.
		{
			Name:     "R2.2_reverse_order_rns",
			Args:     []string{"-rns"},
			ExitCode: 0,
		},
		// R1.2: --kernel-name long form.
		{
			Name:     "R1.2_long_kernel_name",
			Args:     []string{"--kernel-name"},
			ExitCode: 0,
		},
		// R1.3: --nodename long form.
		{
			Name:     "R1.3_long_nodename",
			Args:     []string{"--nodename"},
			ExitCode: 0,
		},
		// R1.4: --kernel-release long form.
		{
			Name:     "R1.4_long_kernel_release",
			Args:     []string{"--kernel-release"},
			ExitCode: 0,
		},
		// Duplicate flags produce same output as single flag.
		{
			Name:     "duplicate_flag_ss",
			Args:     []string{"-s", "-s"},
			ExitCode: 0,
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
