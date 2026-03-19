// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd044-uname R1.1 (default), R1.2 (-s),
// R1.3 (-n), R1.4 (-r).
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
		{
			Name: "default_no_flags",
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "flag_s_kernel_name",
			Args: []string{"-s"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "flag_n_nodename",
			Args: []string{"-n"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "flag_r_kernel_release",
			Args: []string{"-r"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "combined_sn",
			Args: []string{"-sn"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "combined_sr",
			Args: []string{"-sr"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "combined_nr",
			Args: []string{"-nr"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "combined_snr",
			Args: []string{"-snr"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "separate_flags_s_n",
			Args: []string{"-s", "-n"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "separate_flags_n_r",
			Args: []string{"-n", "-r"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "duplicate_flag_ss",
			Args: []string{"-s", "-s"},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
