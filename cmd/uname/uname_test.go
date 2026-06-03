// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var defaultEnv = []string{"LC_ALL=C"}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guname")
	if err != nil {
		t.Skip("reference binary not found")
	}

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?uname`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("uname"))
	})

	tests := []testutils.DiffTest{
		// R1.1, R4.2: default invocation (no flags, equivalent to -s)
		{
			Name: "default-no-flags",
			Env:  defaultEnv,
		},
		// R1.2, R4.2: kernel name
		{
			Name: "flag-s",
			Args: []string{"-s"},
			Env:  defaultEnv,
		},
		// R1.3, R4.2: network node hostname
		{
			Name: "flag-n",
			Args: []string{"-n"},
			Env:  defaultEnv,
		},
		// R1.4, R4.2: kernel release
		{
			Name: "flag-r",
			Args: []string{"-r"},
			Env:  defaultEnv,
		},
		// R1.5, R4.2: kernel version
		{
			Name: "flag-v",
			Args: []string{"-v"},
			Env:  defaultEnv,
		},
		// R1.6, R4.2: machine hardware name
		{
			Name: "flag-m",
			Args: []string{"-m"},
			Env:  defaultEnv,
		},
		// R1.7, R4.2: processor type
		{
			Name: "flag-p",
			Args: []string{"-p"},
			Env:  defaultEnv,
		},
		// R1.8, R4.2: hardware platform
		{
			Name: "flag-i",
			Args: []string{"-i"},
			Env:  defaultEnv,
		},
		// R1.9, R4.2: operating system
		{
			Name: "flag-o",
			Args: []string{"-o"},
			Env:  defaultEnv,
		},
		// R2.1, R4.2: all fields
		{
			Name: "flag-a",
			Args: []string{"-a"},
			Env:  defaultEnv,
		},
		// R2.2, R4.2: combination of two flags
		{
			Name: "combo-sn",
			Args: []string{"-sn"},
			Env:  defaultEnv,
		},
		// R2.2, R4.2: combination of multiple flags
		{
			Name: "combo-snrvm",
			Args: []string{"-snrvm"},
			Env:  defaultEnv,
		},
		// R2.2, R4.2: flags as separate arguments
		{
			Name: "combo-separate-s-r",
			Args: []string{"-s", "-r"},
			Env:  defaultEnv,
		},
		// R2.2, R4.2: combination with -o
		{
			Name: "combo-mo",
			Args: []string{"-mo"},
			Env:  defaultEnv,
		},
		// R3.1, R4.2: extra operand with no flags
		{
			Name:      "error-extra-operand",
			Args:      []string{"foo"},
			ExitCode:  1,
			Env:       defaultEnv,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// R3.1, R4.2: extra operand after valid flag
		{
			Name:      "error-extra-operand-after-flag",
			Args:      []string{"-s", "foo"},
			ExitCode:  1,
			Env:       defaultEnv,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// R3.1, R4.2: extra operand after -- separator
		{
			Name:      "error-operand-after-separator",
			Args:      []string{"--", "foo"},
			ExitCode:  1,
			Env:       defaultEnv,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// R3.2, R4.2: unknown short flag
		{
			Name:      "error-unknown-short-flag",
			Args:      []string{"-z"},
			ExitCode:  1,
			Env:       defaultEnv,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// R3.2, R4.2: unknown long flag
		{
			Name:      "error-unknown-long-flag",
			Args:      []string{"--bogus"},
			ExitCode:  1,
			Env:       defaultEnv,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
