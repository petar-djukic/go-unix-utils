// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd041-id R1.1 (default format), R1.2 (groups),
// R1.3 (exit codes), R2.1 (-u/--user), R2.2 (-g/--group),
// R2.3 (-G/--groups), R2.4 (conflicting selection flags),
// R3.1 (-n/--name modifier).
// R4.1: compare Go vs gid reference binary.
package main

import (
	"os/exec"
	"os/user"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes stderr differences between Go and GNU id:
// replaces binary name/path at the start of lines with "id" and strips
// "Try ... for more information." lines.
var stderrNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	nameRe := regexp.MustCompile(`(?m)^[^\s:]+`)
	data = nameRe.ReplaceAll(data, []byte("id"))
	tryRe := regexp.MustCompile(`(?m)^Try .* for more information\.\n`)
	data = tryRe.ReplaceAll(data, nil)
	// Strip system error suffix after "no such user" (e.g., ": Invalid argument").
	suffixRe := regexp.MustCompile(`(no such user):[^\n]*`)
	data = suffixRe.ReplaceAll(data, []byte("$1"))
	return data
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gid")
	if err != nil {
		t.Skipf("reference binary gid not in PATH: %v", err)
	}
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("cannot get current user: %v", err)
	}
	env := []string{"LC_ALL=C"}
	errNorm := []testutils.NormalizeFunc{stderrNormalizer}
	tests := []testutils.DiffTest{
		// R1.1, R1.2, R1.3: default output — uid, gid, groups.
		{
			Name: "default_no_args",
			Env:  env,
		},
		// R1.1, R1.2: named user (current user).
		{
			Name: "named_user_self",
			Args: []string{currentUser.Username},
			Env:  env,
		},
		// R1.1, R1.2: named user (root).
		{
			Name: "named_user_root",
			Args: []string{"root"},
			Env:  env,
		},
		// R1.3: nonexistent user — exit 1.
		{
			Name:      "nonexistent_user",
			Args:      []string{"no_such_user_12345"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R2.1: -u flag — effective UID.
		{
			Name: "flag_u_short",
			Args: []string{"-u"},
			Env:  env,
		},
		// R2.1: --user long flag.
		{
			Name: "flag_user_long",
			Args: []string{"--user"},
			Env:  env,
		},
		// R2.1: -u with named user (root).
		{
			Name: "flag_u_named_root",
			Args: []string{"-u", "root"},
			Env:  env,
		},
		// R2.2: -g flag — effective GID.
		{
			Name: "flag_g_short",
			Args: []string{"-g"},
			Env:  env,
		},
		// R2.2: --group long flag.
		{
			Name: "flag_group_long",
			Args: []string{"--group"},
			Env:  env,
		},
		// R2.2: -g with named user (root).
		{
			Name: "flag_g_named_root",
			Args: []string{"-g", "root"},
			Env:  env,
		},
		// R2.3: -G flag — all groups space-separated.
		{
			Name: "flag_G_short",
			Args: []string{"-G"},
			Env:  env,
		},
		// R2.3: --groups long flag.
		{
			Name: "flag_groups_long",
			Args: []string{"--groups"},
			Env:  env,
		},
		// R2.3: -G with named user (root).
		{
			Name: "flag_G_named_root",
			Args: []string{"-G", "root"},
			Env:  env,
		},
		// R2.4: conflicting -u and -g — error, exit 1.
		{
			Name:      "conflict_u_g",
			Args:      []string{"-u", "-g"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R2.4: conflicting -u and -G — error, exit 1.
		{
			Name:      "conflict_u_G",
			Args:      []string{"-u", "-G"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R2.4: conflicting -g and -G — error, exit 1.
		{
			Name:      "conflict_g_G",
			Args:      []string{"-g", "-G"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R3.1: -un — effective user name.
		{
			Name: "flag_un",
			Args: []string{"-un"},
			Env:  env,
		},
		// R3.1: -gn — effective group name.
		{
			Name: "flag_gn",
			Args: []string{"-gn"},
			Env:  env,
		},
		// R3.1: -Gn — all group names.
		{
			Name: "flag_Gn",
			Args: []string{"-Gn"},
			Env:  env,
		},
		// R3.1: -n alone — error, exit 1.
		{
			Name:      "flag_n_alone",
			Args:      []string{"-n"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R3.1: -gn with named user (root).
		{
			Name: "flag_gn_named_root",
			Args: []string{"-gn", "root"},
			Env:  env,
		},
		// R3.1: -Gn with named user (current user).
		{
			Name: "flag_Gn_named_self",
			Args: []string{"-Gn", currentUser.Username},
			Env:  env,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
