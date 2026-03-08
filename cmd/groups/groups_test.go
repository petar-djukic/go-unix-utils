// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/groups against the GNU reference binary (ggroups).
//
// Implements prd043-groups acceptance criteria AC1-AC6 via testutils.RunDiffTests.
package main

import (
	"bytes"
	"os/exec"
	"os/user"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrBinaryNameNormalizer replaces "ggroups:" with "groups:" so stderr
// comparison ignores the binary name difference.
var stderrBinaryNameNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("ggroups:"), []byte("groups:"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ggroups")
	if err != nil {
		t.Skipf("reference binary ggroups not in PATH: %v", err)
	}

	u, err := user.Current()
	if err != nil {
		t.Fatalf("cannot get current user: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: No arguments prints current user groups.
		{
			Name: "groups_no_args",
			Args: []string{},
		},
		// R1.2: Named user prints groups with prefix.
		{
			Name: "groups_named_user",
			Args: []string{u.Username},
		},
		// R1.3: Nonexistent user causes error.
		{
			Name:      "groups_nonexistent_user",
			Args:      []string{"surely_nonexistent_user_xyz"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrBinaryNameNormalizer},
		},
		// R1.2/R1.3: Mix of valid and invalid users.
		{
			Name:      "groups_mixed_users",
			Args:      []string{u.Username, "surely_nonexistent_user_xyz"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrBinaryNameNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
