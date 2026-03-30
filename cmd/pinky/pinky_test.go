// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// pinky_test.go implements differential tests for cmd/pinky against gpinky.
// Covers prd098-pinky R1.1-R1.3, R2.1-R2.3, R3.1-R3.3.

package main_test

import (
	"os/exec"
	"os/user"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// binaryNameRe matches gpinky or pinky with optional path prefix.
var binaryNameRe = regexp.MustCompile(`(/[^ ']*/)?(g?pinky)`)

// normalizeBinaryName replaces binary path references with "pinky"
// so stderr from gpinky and our binary can be compared.
func normalizeBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("pinky"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpinky")
	if err != nil {
		t.Skip("reference binary gpinky not in PATH")
	}

	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("cannot determine current user: %v", err)
	}
	username := currentUser.Username

	errorNorm := []testutils.NormalizeFunc{normalizeBinaryName}

	tests := []testutils.DiffTest{
		// R1.1: default short format listing.
		{
			Name: "pinky_default",
			Args: []string{},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: -s forces short format (same as default).
		{
			Name: "pinky_short",
			Args: []string{"-s"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2, R2 task: username argument filters to that user.
		{
			Name: "pinky_user",
			Args: []string{username},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -l long format for current user.
		{
			Name: "pinky_long",
			Args: []string{"-l", username},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: -f suppresses header in short format.
		{
			Name: "pinky_no_header",
			Args: []string{"-f"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: -b suppresses home dir and shell in long format.
		{
			Name: "pinky_long_no_dir",
			Args: []string{"-lb", username},
			Env:  []string{"LC_ALL=C"},
		},
		// Combined -sf (short format, no header).
		{
			Name: "pinky_short_no_header",
			Args: []string{"-sf"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -l without users requires at least one username.
		{
			Name:      "pinky_long_no_users",
			Args:      []string{"-l"},
			ExitCode:  1,
			Normalize: errorNorm,
		},
		// R2.3: -h and -p flags accepted (suppress project/plan).
		{
			Name: "pinky_long_hp",
			Args: []string{"-lhp", username},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1, R3.2: exit 1 for unknown flag.
		{
			Name:      "pinky_unknown_flag",
			Args:      []string{"-x"},
			ExitCode:  1,
			Normalize: errorNorm,
		},
		// R3.2: exit 1 for another invalid flag.
		{
			Name:      "pinky_invalid_flag_z",
			Args:      []string{"-z"},
			ExitCode:  1,
			Normalize: errorNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
