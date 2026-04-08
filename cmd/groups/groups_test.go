// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/groups.
// Tests cover srd043-groups R3.1, R3.2, R3.3.
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

// stderrNormalizer normalizes program name differences in error messages.
// R3.1/R3.2: replaces binary paths with "PROG" so error message structure
// can be compared between Go and GNU binaries.
func stderrNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	b = stderrProgRe.ReplaceAll(b, []byte("PROG:"))
	b = stderrTryRe.ReplaceAll(b, []byte("'PROG --help'"))
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
	refBin, err := exec.LookPath("ggroups")
	if err != nil {
		t.Skipf("reference binary ggroups not in PATH: %v", err)
	}

	username := currentUsername(t)

	tests := []testutils.DiffTest{
		// R3.1: default invocation with no arguments prints current
		// user's group memberships.
		{
			Name: "default_no_args",
			Args: []string{},
		},

		// R3.2/R3.3: single named user prints "user : groups" format.
		{
			Name: "named_current_user",
			Args: []string{username},
		},

		// R3.2/R3.3: multiple named users, one line per user with prefix.
		{
			Name: "multiple_named_users",
			Args: []string{username, username},
		},

		// R3.2: nonexistent user prints error to stderr and exits 1.
		{
			Name:      "nonexistent_user",
			Args:      []string{"no_such_user_xyzzy_42"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R3.2: mixed valid and invalid users — error for invalid,
		// output for valid, exit 1 overall.
		{
			Name:      "mixed_valid_invalid",
			Args:      []string{username, "no_such_user_xyzzy_42"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R3.3: --help flag exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R3.3: --version flag exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R3.2: unknown flag produces error and exit 1.
		{
			Name:      "unknown_flag",
			Args:      []string{"--bogus"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
