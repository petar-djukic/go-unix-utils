// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd041-id R1.1 (default format), R1.2 (groups),
// R1.3 (exit codes), R2.1 (-u/--user), R2.2 (-g/--group),
// R2.3 (-G/--groups), R2.4 (conflicting selection flags),
// R3.1 (-n/--name modifier), R3.2 (-r/--real modifier),
// R3.3 (USER operand named user lookup).
// R4.1: compare Go vs gid reference binary.
// R4.2: covers default, -u, -g, -G, -n, -r, named user, nonexistent user.
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
		// R1.1, R1.2, R3.3: named user (current user).
		{
			Name: "named_user_self",
			Args: []string{currentUser.Username},
			Env:  env,
		},
		// R1.1, R1.2, R3.3: named user (root).
		{
			Name: "named_user_root",
			Args: []string{"root"},
			Env:  env,
		},
		// R3.3: nonexistent user — exit 1.
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
		// R2.1, R3.3: -u with named user (root).
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
		// R2.2, R3.3: -g with named user (root).
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
		// R2.3, R3.3: -G with named user (root).
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
		// R3.1, R3.3: -gn with named user (root).
		{
			Name: "flag_gn_named_root",
			Args: []string{"-gn", "root"},
			Env:  env,
		},
		// R3.1, R3.3: -Gn with named user (current user).
		{
			Name: "flag_Gn_named_self",
			Args: []string{"-Gn", currentUser.Username},
			Env:  env,
		},
		// R3.2: -ru — real UID (numeric).
		{
			Name: "flag_ru",
			Args: []string{"-ru"},
			Env:  env,
		},
		// R3.2: -rg — real GID (numeric).
		{
			Name: "flag_rg",
			Args: []string{"-rg"},
			Env:  env,
		},
		// R3.2: -run — real user name.
		{
			Name: "flag_run",
			Args: []string{"-run"},
			Env:  env,
		},
		// R3.2: -rgn — real group name.
		{
			Name: "flag_rgn",
			Args: []string{"-rgn"},
			Env:  env,
		},
		// R3.2: --real --user long flags.
		{
			Name: "flag_real_user_long",
			Args: []string{"--real", "--user"},
			Env:  env,
		},
		// R3.2: --real --group long flags.
		{
			Name: "flag_real_group_long",
			Args: []string{"--real", "--group"},
			Env:  env,
		},
		// R3.2: -r alone — error, exit 1.
		{
			Name:      "flag_r_alone",
			Args:      []string{"-r"},
			Env:       env,
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R3.2: -rG — -r is accepted with -G (groups have no real/effective distinction).
		{
			Name: "flag_rG",
			Args: []string{"-rG"},
			Env:  env,
		},
		// R3.2, R3.3: -ru with named user (root).
		{
			Name: "flag_ru_named_root",
			Args: []string{"-ru", "root"},
			Env:  env,
		},
		// R3.3: -un with named user (current user).
		{
			Name: "flag_un_named_self",
			Args: []string{"-un", currentUser.Username},
			Env:  env,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
