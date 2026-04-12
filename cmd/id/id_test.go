// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/id.
// Tests cover srd041-id R4.1, R4.2, R4.3.
package main

import (
	"os/exec"
	"os/user"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgRe matches the program name/path prefix before a colon at line start.
var stderrProgRe = regexp.MustCompile(`(?m)^[^\s:]+:`)

// stderrTryRe matches the quoted program reference in Try hint lines.
var stderrTryRe = regexp.MustCompile(`'[^']*--help'`)

// noSuchUserSuffixRe strips OS-specific suffix after "no such user" in error messages.
// GNU id appends ": Invalid argument" from strerror; Go omits it.
var noSuchUserSuffixRe = regexp.MustCompile(`no such user[^\n]*`)

// stderrNormalizer normalizes program name differences in error messages.
// R4.3: replaces binary paths with "PROG" so error message structure
// can be compared between Go and GNU binaries.
func stderrNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	b = stderrProgRe.ReplaceAll(b, []byte("PROG:"))
	b = stderrTryRe.ReplaceAll(b, []byte("'PROG --help'"))
	b = noSuchUserSuffixRe.ReplaceAll(b, []byte("no such user"))
	return b
}

// discardOutput normalizes by discarding all output, used when
// output content differs by design (--version, --help) and only
// exit code comparison is meaningful.
func discardOutput(b []byte) []byte {
	return nil
}

// currentUsername returns the current user's login name for test cases.
func currentUsername(t *testing.T) string {
	t.Helper()
	u, err := user.Current()
	if err != nil {
		t.Fatalf("cannot determine current user: %v", err)
	}
	return u.Username
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gid")
	if err != nil {
		t.Skipf("reference binary gid not in PATH: %v", err)
	}

	username := currentUsername(t)

	tests := []testutils.DiffTest{
		// R4.1: default invocation with no arguments prints
		// uid=N(name) gid=N(name) groups=...
		{
			Name: "default_no_args",
			Args: []string{},
		},

		// R4.1: -u prints effective UID as a number.
		{
			Name: "flag_u",
			Args: []string{"-u"},
		},

		// R4.1: -g prints effective GID as a number.
		{
			Name: "flag_g",
			Args: []string{"-g"},
		},

		// R4.1: -G prints all group IDs space-separated.
		{
			Name: "flag_G",
			Args: []string{"-G"},
		},

		// R4.1: -un prints effective username.
		{
			Name: "flag_un",
			Args: []string{"-un"},
		},

		// R4.1: -gn prints effective group name.
		{
			Name: "flag_gn",
			Args: []string{"-gn"},
		},

		// R4.1: -Gn prints all group names space-separated.
		{
			Name: "flag_Gn",
			Args: []string{"-Gn"},
		},

		// R4.1: -ru prints real UID.
		{
			Name: "flag_ru",
			Args: []string{"-ru"},
		},

		// R4.1: -rg prints real GID.
		{
			Name: "flag_rg",
			Args: []string{"-rg"},
		},

		// R4.1: -run prints real username.
		{
			Name: "flag_run",
			Args: []string{"-run"},
		},

		// R4.1: -rgn prints real group name.
		{
			Name: "flag_rgn",
			Args: []string{"-rgn"},
		},

		// R4.2: named user (current user) prints identity info.
		{
			Name: "named_current_user",
			Args: []string{username},
		},

		// R4.2: named user with -u flag.
		{
			Name: "named_user_flag_u",
			Args: []string{"-u", username},
		},

		// R4.2: named user with -gn flag.
		{
			Name: "named_user_flag_gn",
			Args: []string{"-gn", username},
		},

		// R4.2: named user with -G flag.
		{
			Name: "named_user_flag_G",
			Args: []string{"-G", username},
		},

		// R4.2: named user with -Gn flag.
		{
			Name: "named_user_flag_Gn",
			Args: []string{"-Gn", username},
		},

		// R4.2: root user lookup.
		{
			Name: "named_user_root",
			Args: []string{"root"},
		},

		// R4.2: nonexistent user produces error and exit 1.
		{
			Name:      "nonexistent_user",
			Args:      []string{"no_such_user_xyzzy_42"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R4.3: -n without a selection flag is an error.
		{
			Name:      "flag_n_alone",
			Args:      []string{"-n"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R4.3: -r without -u or -g is an error.
		{
			Name:      "flag_r_alone",
			Args:      []string{"-r"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R4.3: conflicting -u and -g flags.
		{
			Name:      "conflicting_u_g",
			Args:      []string{"-u", "-g"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R4.3: conflicting -u and -G flags.
		{
			Name:      "conflicting_u_G",
			Args:      []string{"-u", "-G"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R4.3: -r with -G is silently accepted by GNU id (ignores -r).
		{
			Name: "flag_r_with_G",
			Args: []string{"-rG"},
		},

		// R4.3: --help flag exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R4.3: --version flag exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R4.3: unknown flag produces error and exit 1.
		{
			Name:      "unknown_flag",
			Args:      []string{"--bogus"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R4.3: -z with -G prints NUL-delimited groups.
		{
			Name: "flag_z_with_G",
			Args: []string{"-zG"},
		},

		// R4.3: -z with -u prints UID with NUL terminator.
		{
			Name: "flag_z_with_u",
			Args: []string{"-zu"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
