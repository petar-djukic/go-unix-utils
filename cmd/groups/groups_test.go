// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd043-groups R3.1–R3.3: differential tests for groups.
package main

import (
	"os/exec"
	"os/user"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes stderr differences between Go and GNU groups:
// replaces binary name/path at the start of lines with "groups" and strips
// "Try ... for more information." lines.
var stderrNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	tryRe := regexp.MustCompile(`(?m)^Try .* for more information\.\n`)
	data = tryRe.ReplaceAll(data, nil)
	nameRe := regexp.MustCompile(`(?m)^[^\s:]+`)
	data = nameRe.ReplaceAll(data, []byte("groups"))
	return data
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ggroups")
	if err != nil {
		t.Skipf("reference binary ggroups not in PATH: %v", err)
	}
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("cannot get current user: %v", err)
	}
	username := currentUser.Username
	errNorm := []testutils.NormalizeFunc{stderrNormalizer}
	tests := []testutils.DiffTest{
		{
			// R3.2: no-argument current-user output.
			Name:     "no_args",
			Args:     []string{},
			ExitCode: 0,
		},
		{
			// R3.2, R3.3: single named user with "user :" prefix.
			Name:     "named_current_user",
			Args:     []string{username},
			ExitCode: 0,
		},
		{
			// R3.2: nonexistent user error.
			Name:      "nonexistent_user",
			Args:      []string{"nonexistent_user_xyz_12345"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			// R3.2: mixed valid and invalid users.
			Name:      "mixed_valid_invalid",
			Args:      []string{username, "nonexistent_user_xyz_12345"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			// R3.2, R3.3: multiple valid named users.
			Name:     "multiple_named_users",
			Args:     []string{username, username},
			ExitCode: 0,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
