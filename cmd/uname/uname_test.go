// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd044-uname R3.1, R3.2, R4.1, R4.2:
// compare Go uname against guname reference binary for individual flags,
// combined flags, -a, and error conditions.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// binaryNameNormalizer replaces the reference binary name and path in
// stderr so that "guname" and "/opt/.../guname" both become "uname".
var binaryNameNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	re := regexp.MustCompile(`[^\s']*g?uname`)
	return re.ReplaceAll(data, []byte("uname"))
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guname")
	if err != nil {
		t.Skipf("reference binary guname not in PATH: %v", err)
	}
	env := []string{"LC_ALL=C"}
	errNorm := []testutils.NormalizeFunc{binaryNameNormalizer}
	tests := []testutils.DiffTest{
		// R4.2: default (no flags) — prints kernel name (R1.1).
		{
			Name: "default_no_flags",
			Env:  env,
		},
		// R4.2: individual flag -s — kernel name (R1.2).
		{
			Name: "flag_s_kernel_name",
			Args: []string{"-s"},
			Env:  env,
		},
		// R4.2: individual flag -n — node hostname (R1.3).
		{
			Name: "flag_n_nodename",
			Args: []string{"-n"},
			Env:  env,
		},
		// R4.2: individual flag -r — kernel release (R1.4).
		{
			Name: "flag_r_kernel_release",
			Args: []string{"-r"},
			Env:  env,
		},
		// R4.2: individual flag -v — kernel version (R1.5).
		{
			Name: "flag_v_kernel_version",
			Args: []string{"-v"},
			Env:  env,
		},
		// R4.2: individual flag -m — machine hardware name (R1.6).
		{
			Name: "flag_m_machine",
			Args: []string{"-m"},
			Env:  env,
		},
		// R4.2: individual flag -p — processor type (R1.7).
		{
			Name: "flag_p_processor",
			Args: []string{"-p"},
			Env:  env,
		},
		// R4.2: individual flag -i — hardware platform (R1.8).
		{
			Name: "flag_i_platform",
			Args: []string{"-i"},
			Env:  env,
		},
		// R4.2: individual flag -o — operating system (R1.9).
		{
			Name: "flag_o_operating_system",
			Args: []string{"-o"},
			Env:  env,
		},
		// R4.2: combined -a — all fields in canonical order (R2.1).
		{
			Name: "flag_a_all",
			Args: []string{"-a"},
			Env:  env,
		},
		// R4.1, R4.2: flag combination -sn (R2.2).
		{
			Name: "combined_sn",
			Args: []string{"-sn"},
			Env:  env,
		},
		// R4.1, R4.2: flag combination -sr (R2.2).
		{
			Name: "combined_sr",
			Args: []string{"-sr"},
			Env:  env,
		},
		// R4.1, R4.2: flag combination -snrvm (R2.2).
		{
			Name: "combined_snrvm",
			Args: []string{"-snrvm"},
			Env:  env,
		},
		// R4.2: separate flags -s -n produce same as -sn.
		{
			Name: "separate_flags_s_n",
			Args: []string{"-s", "-n"},
			Env:  env,
		},
		// R4.2: duplicate flag is idempotent.
		{
			Name: "duplicate_flag_ss",
			Args: []string{"-s", "-s"},
			Env:  env,
		},
		// R4.2: error — invalid short option (R3.2).
		{
			Name:      "invalid_short_option",
			Args:      []string{"-x"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R4.2: error — invalid long option (R3.2).
		{
			Name:      "invalid_long_option",
			Args:      []string{"--unknown"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
