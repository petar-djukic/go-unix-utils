// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/id against GNU gid.
// Covers prd041-id R4.1 (differential testing), R4.2 (flag coverage),
// R4.3 (error cases and exit codes).
package main

import (
	"os/exec"
	"os/user"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes error messages between GNU gid and the Go binary
// so differences in binary paths, "Try" hints, and system error suffixes
// do not cause false failures.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?id|gid`)
	tryHelp := regexp.MustCompile(`(?m)^Try '[^']*' for more information\.\n?`)
	// GNU appends system-specific text after "no such user" (e.g., ": Invalid argument").
	noSuchUserSuffix := regexp.MustCompile(`(no such user):[^\n]*`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("id"))
		b = tryHelp.ReplaceAll(b, nil)
		b = noSuchUserSuffix.ReplaceAll(b, []byte("$1"))
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gid")
	if err != nil {
		t.Skipf("reference binary gid not in PATH: %v", err)
	}

	// Resolve current username for named-user tests.
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("cannot determine current user: %v", err)
	}
	username := currentUser.Username

	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		// R4.1: default output (no flags) — uid=N(name) gid=N(name) groups=...
		{
			Name: "default_no_flags",
		},
		// R4.2: -u prints effective UID.
		{
			Name: "user_flag",
			Args: []string{"-u"},
		},
		// R4.2: -g prints effective GID.
		{
			Name: "group_flag",
			Args: []string{"-g"},
		},
		// R4.2: -G prints all group IDs space-separated.
		{
			Name: "groups_flag",
			Args: []string{"-G"},
		},
		// R4.2: -un prints effective username.
		{
			Name: "user_name_flag",
			Args: []string{"-un"},
		},
		// R4.2: -gn prints effective group name.
		{
			Name: "group_name_flag",
			Args: []string{"-gn"},
		},
		// R4.2: -Gn prints all group names space-separated.
		{
			Name: "groups_name_flag",
			Args: []string{"-Gn"},
		},
		// R4.2: -ur prints real UID.
		{
			Name: "user_real_flag",
			Args: []string{"-ur"},
		},
		// R4.2: -gr prints real GID.
		{
			Name: "group_real_flag",
			Args: []string{"-gr"},
		},
		// R4.2: -urn prints real username by name.
		{
			Name: "user_real_name_flag",
			Args: []string{"-urn"},
		},
		// R4.2: named user lookup with current username.
		{
			Name: "named_user",
			Args: []string{username},
		},
		// R4.2: named user with -u flag.
		{
			Name: "named_user_u",
			Args: []string{"-u", username},
		},
		// R4.2: named user with -Gn flag.
		{
			Name: "named_user_Gn",
			Args: []string{"-Gn", username},
		},
		// R4.3: nonexistent user — exit 1.
		{
			Name:      "nonexistent_user",
			Args:      []string{"no_such_user_xyzzy_99"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3: conflicting selection flags -u and -g — exit 1.
		{
			Name:      "conflict_u_g",
			Args:      []string{"-u", "-g"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3: -n without selection flag — exit 1.
		{
			Name:      "name_without_selection",
			Args:      []string{"-n"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3: -r without -u or -g — exit 1.
		{
			Name:      "real_without_selection",
			Args:      []string{"-r"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3: invalid short flag — exit 1.
		{
			Name:      "invalid_short_flag",
			Args:      []string{"-x"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
