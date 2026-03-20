// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd041-id R1.1 (default format), R1.2 (groups),
// R1.3 (exit codes), R2.1 (-u/--user flag).
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
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
