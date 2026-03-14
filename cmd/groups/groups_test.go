// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd043-groups R3.1, R3.2, R3.3 (differential tests)
package main

import (
	"os/exec"
	"os/user"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for groups.
const refBinaryName = "ggroups"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// normalizeProgramName replaces the reference binary path/name with a
	// fixed token so stderr messages compare equal despite different argv[0].
	programNamePattern := regexp.MustCompile(`(?:` + regexp.QuoteMeta(refBin) + `|ggroups|groups)`)
	normalizeProgramName := func(b []byte) []byte {
		return programNamePattern.ReplaceAll(b, []byte("PROG"))
	}

	stderrNorm := []testutils.NormalizeFunc{normalizeProgramName}

	// Get the current username for named-user tests.
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("cannot determine current user: %v", err)
	}
	currentUsername := currentUser.Username

	tests := []testutils.DiffTest{
		// R3.2: no-argument current-user output.
		{
			Name: "no_args",
			Args: []string{},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2, R3.3: single named user — verifies "user :" prefix.
		{
			Name: "current_user_named",
			Args: []string{currentUsername},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: nonexistent user — exit 1 with error on stderr.
		{
			Name:      "nonexistent_user",
			Args:      []string{"nonexistent_user_xyz_99"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// R3.2: multiple named users.
		{
			Name: "multiple_users",
			Args: []string{currentUsername, currentUsername},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: mixed valid and invalid users — exit 1, valid users still printed.
		{
			Name:      "mixed_valid_invalid",
			Args:      []string{currentUsername, "nonexistent_user_xyz_99"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffHelpVersion verifies --help and --version exit 0.
// Output content differs between implementations, so stdout/stderr are
// normalized to empty; only exit codes are compared.
func TestDiffHelpVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	clearOutput := func(b []byte) []byte { return nil }

	tests := []testutils.DiffTest{
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
