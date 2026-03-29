// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/uname against guname (GNU coreutils).
//
// Covers prd044-uname R3.2, R4.1, R4.2, R4.3.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
// Used for error messages where GNU includes the full binary path
// in the program name prefix, causing unavoidable divergence.
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guname")
	if err != nil {
		t.Skip("reference binary guname not in PATH")
	}

	// R4.3: all tests set LC_ALL=C.
	env := []string{"LC_ALL=C"}

	tests := []testutils.DiffTest{
		// R4.1, R4.2: default (no flags) — prints kernel name.
		{
			Name:     "default_no_args",
			Args:     []string{},
			Env:      env,
			ExitCode: 0,
		},
		// R4.1, R4.2: -s prints kernel name.
		{
			Name:     "flag_s_kernel_name",
			Args:     []string{"-s"},
			Env:      env,
			ExitCode: 0,
		},
		// R4.1, R4.2: -n prints network node hostname.
		{
			Name:     "flag_n_nodename",
			Args:     []string{"-n"},
			Env:      env,
			ExitCode: 0,
		},
		// R4.1, R4.2: -r prints kernel release.
		{
			Name:     "flag_r_kernel_release",
			Args:     []string{"-r"},
			Env:      env,
			ExitCode: 0,
		},
		// R4.1, R4.2: -v prints kernel version.
		{
			Name:     "flag_v_kernel_version",
			Args:     []string{"-v"},
			Env:      env,
			ExitCode: 0,
		},
		// R4.1, R4.2: -m prints machine hardware name.
		{
			Name:     "flag_m_machine",
			Args:     []string{"-m"},
			Env:      env,
			ExitCode: 0,
		},
		// R4.1, R4.2: -p prints processor type.
		{
			Name:     "flag_p_processor",
			Args:     []string{"-p"},
			Env:      env,
			ExitCode: 0,
		},
		// R4.1, R4.2: -i prints hardware platform.
		{
			Name:     "flag_i_hardware_platform",
			Args:     []string{"-i"},
			Env:      env,
			ExitCode: 0,
		},
		// R4.1, R4.2: -o prints operating system name.
		{
			Name:     "flag_o_os_name",
			Args:     []string{"-o"},
			Env:      env,
			ExitCode: 0,
		},
		// R4.1, R4.2: -a prints all fields in canonical order.
		{
			Name:     "flag_a_all",
			Args:     []string{"-a"},
			Env:      env,
			ExitCode: 0,
		},
		// R4.2: combined flags -sn — kernel name and nodename.
		{
			Name:     "combined_sn",
			Args:     []string{"-sn"},
			Env:      env,
			ExitCode: 0,
		},
		// R4.2: combined flags -mr — machine and kernel release.
		{
			Name:     "combined_mr",
			Args:     []string{"-mr"},
			Env:      env,
			ExitCode: 0,
		},
		// R4.2: combined flags -smo — kernel name, machine, os.
		{
			Name:     "combined_smo",
			Args:     []string{"-smo"},
			Env:      env,
			ExitCode: 0,
		},
		// R4.2: error case — extra operand.
		{
			Name:      "extra_operand_error",
			Args:      []string{"extraarg"},
			Env:       env,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: error case — unknown flag.
		{
			Name:      "unknown_flag_error",
			Args:      []string{"-Z"},
			Env:       env,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestHelp verifies --help prints to stdout and exits 0 (R3.2).
func TestHelp(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("--help produced no output")
	}
}
