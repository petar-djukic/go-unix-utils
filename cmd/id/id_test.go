// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/id against the GNU reference binary (gid).
//
// Implements prd041-id acceptance criteria AC1-AC6 via testutils.RunDiffTests.
package main

import (
	"bytes"
	"os/exec"
	"os/user"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer replaces "gid:" with "id:" and strips trailing system
// error details (e.g., ": Invalid argument") that differ across platforms.
var stderrNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gid:"), []byte("id:"))
	// GNU id appends the libc error (e.g., ": Invalid argument") after "no such user".
	// Go's user.Lookup does not. Normalize by removing everything after "no such user".
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		if idx := bytes.Index(line, []byte("no such user")); idx >= 0 {
			lines[i] = append(line[:idx+len("no such user")], '\n')
			lines[i] = bytes.TrimRight(lines[i], "\n")
		}
	}
	return bytes.Join(lines, []byte("\n"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gid")
	if err != nil {
		t.Skipf("reference binary gid not in PATH: %v", err)
	}

	u, err := user.Current()
	if err != nil {
		t.Fatalf("cannot get current user: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: Default output format.
		{
			Name: "id_default",
			Args: []string{},
		},
		// R2.1: -u prints effective UID.
		{
			Name: "id_user_numeric",
			Args: []string{"-u"},
		},
		// R2.2: -g prints effective GID.
		{
			Name: "id_group_numeric",
			Args: []string{"-g"},
		},
		// R2.3: -G prints all group IDs.
		{
			Name: "id_groups_numeric",
			Args: []string{"-G"},
		},
		// R3.1: -un prints username.
		{
			Name: "id_user_name",
			Args: []string{"-un"},
		},
		// R3.1: -gn prints group name.
		{
			Name: "id_group_name",
			Args: []string{"-gn"},
		},
		// R3.1: -Gn prints group names.
		{
			Name: "id_groups_names",
			Args: []string{"-Gn"},
		},
		// R3.2: -ru prints real UID.
		{
			Name: "id_real_user",
			Args: []string{"-ru"},
		},
		// R3.2: -rg prints real GID.
		{
			Name: "id_real_group",
			Args: []string{"-rg"},
		},
		// R3.3: Named user.
		{
			Name: "id_named_user",
			Args: []string{u.Username},
		},
		// R3.3: Named user with -u.
		{
			Name: "id_named_user_uid",
			Args: []string{"-u", u.Username},
		},
		// R3.3: Named user with -Gn.
		{
			Name: "id_named_user_groups_names",
			Args: []string{"-Gn", u.Username},
		},
		// R3.3: Nonexistent user.
		{
			Name:      "id_nonexistent_user",
			Args:      []string{"surely_nonexistent_user_xyz"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
