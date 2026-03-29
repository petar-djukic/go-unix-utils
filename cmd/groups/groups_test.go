// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os/exec"
	"os/user"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeProgramName replaces "ggroups:" with "groups:" so stderr
// from the reference binary matches our output.
func normalizeProgramName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("ggroups:"), []byte("groups:"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ggroups")
	if err != nil {
		t.Skipf("reference binary ggroups not in PATH: %v", err)
	}

	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("cannot determine current user: %v", err)
	}

	normalize := []testutils.NormalizeFunc{normalizeProgramName}

	tests := []testutils.DiffTest{
		{
			// R1.1: no arguments prints current user's groups.
			Name: "no_args_current_user",
			Args: []string{},
		},
		{
			// R1.2: single named user with "user :" prefix.
			// R3.3: verifies prefix format.
			Name: "single_named_user",
			Args: []string{currentUser.Username},
		},
		{
			// R1.2: multiple named users, one line each.
			Name: "multiple_named_users",
			Args: []string{currentUser.Username, "root"},
		},
		{
			// R1.3: nonexistent user produces error, exit 1.
			Name:      "nonexistent_user",
			Args:      []string{"nonexistent_user_xyz_999"},
			ExitCode:  1,
			Normalize: normalize,
		},
		{
			// R1.3, R3.2: mixed valid and invalid users.
			Name:      "mixed_valid_invalid",
			Args:      []string{currentUser.Username, "nonexistent_user_xyz_999"},
			ExitCode:  1,
			Normalize: normalize,
		},
		{
			// R3.3: named user root verifies "user :" prefix.
			Name: "root_user",
			Args: []string{"root"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
